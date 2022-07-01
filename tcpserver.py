#!/usr/bin/env python3

import socket, time, datetime
from abc import ABC, abstractmethod
from myeventloop import Timeout, Handler, EventLoop

class TCPServerHandler(Handler):
    def __init__(self, addr, sock):
        super().__init__("%s:%d" % addr, sock, socket.error)
        self.recv_buf = []
        self.send_buf = []

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

    # Called when connection is half-closed i.e. recv() returns 0
    # Override if your protocol uses shutdown() to communicate EOM
    def shutdown_callback(self):
        self.destroy()

    def is_writable(self):
        return not not self.send_buf

    # Use this as a shortcut to add data to send stream queue
    def send(self, data):
        self.send_buf += data

    def write_callback(self):
        self.send_callback()

    def send_callback(self):
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


class TCPListener(Handler):
    # handler_class expected to be subclass of TCPServerHandler
    def __init__(self, addr, handler_class):
        self.handler_class = handler_class 
        fd = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        fd.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        fd.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEPORT, 1)
        fd.bind(addr)
        fd.listen(5)
        super().__init__("listener", fd, socket.error)

    def read_callback(self):
        try:
            client_sock, addr = self.fd.accept()
        except socket.error:
            return
        handler = self.handler_class(addr, client_sock)


class TCPServerEventLoop(EventLoop):
    # listener_class expected to be subclass of TCPListener
    # handler_class expected to be subclass of TCPServerHandler
    def __init__(self, addr, listener_class, handler_class):
        listener = listener_class(addr, handler_class)
        super().__init__()
