#!/usr/bin/env python3

import socket, time, datetime
from abc import ABC, abstractmethod
from myeventloop import Timeout, Handler, EventLoop

class TCPClientHandler(Handler):
    def __init__(self, addr):
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        super().__init__("client %s:%d" % addr, sock, socket.error)

        self.recv_buf = []
        self.send_buf = []

        self.fd.setblocking(0)
        self.connecting = True
        try:
            self.fd.connect(addr)
        except BlockingIOError as e:
            pass
        # will select() for write when connection is up (or failed)

    def read_callback(self):
        try:
            data = self.fd.recv(4096)
        except socket.error as err:
            self.log_warn("exception reading sk", err)
            self.destroy()
            return

        if not data:
            self.shutdown_callback()
            return

        self.recv_buf += data
        self.recv_callback(data)

    # Called when connection receives new data
    # You must override this
    @abstractmethod
    def recv_callback(self, latest):
        pass

    # Called when connection is up, or failed
    # When ok is False, Handler is destroyed right after return,
    # so don't count on further communication or timeouts tied to
    # this Handler (delegate work to ownerless Timeouts if necessary).
    #
    # You must override this
    @abstractmethod
    def connection_callback(self, ok):
        pass

    # Called when connection is half-closed i.e. recv() returns 0
    # Override if your protocol uses shutdown() to communicate EOM
    def shutdown_callback(self):
        self.destroy()

    def is_readable(self):
        return not self.connecting

    def is_writable(self):
        return self.connecting or (not not self.send_buf)

    def write_callback(self):
        self.send_callback()

    # Use this as a shortcut to add data to send stream queue
    def send(self, data):
        self.send_buf += data

    def send_callback(self):
        if self.connecting:
            self._connection_callback()
            return

        try:
            sent = self.fd.send(bytearray(self.send_buf[0:4096]))
        except socket.error as err:
            self.log_warn("exception writing sk", err)
            self.destroy()
            return 0

        if sent <= 0:
            self.destroy()
            return 0

        self.send_buf = self.send_buf[sent:]
        return sent

    def _connection_callback(self):
        self.connecting = False
        if self.fd.getsockopt(socket.SOL_SOCKET, socket.SO_ERROR):
            self.connection_callback(False)
            self.destroy()
            return

        self.fd.setblocking(1)
        self.connection_callback(True)
