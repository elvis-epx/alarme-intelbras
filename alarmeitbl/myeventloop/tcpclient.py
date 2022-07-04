#!/usr/bin/env python3

import socket, time, datetime
from abc import ABC, abstractmethod
from . import Timeout, Handler, EventLoop

class TCPClientHandler(Handler):
    """
    Handler specialization to create, encapsulate and handle active TCP
    connections.

    This is an abstract class, and there are methods you are required
    to override to complete the implementation.
    """
    def __init__(self, addr):
        """
        Instantiate TCP active connection, create the socket and
        start connection process.
        Arguments:
            addr: address/port tuple of the server to connect to
        """
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

    @abstractmethod
    def recv_callback(self, latest):
        """
        Override this abstract method to receive data from TCP connection.

        This class adds all new data to Handler.read_buf variable, so you
        don't need to handle buffering yourself. (And you should drain
        read_buf as data is processed, to save memory.)

        Arguments:
            latest: new data octets just received
        """
        pass

    @abstractmethod
    def connection_callback(self, ok):
        """
        Called when connection is up, or has failed.
        Override if you need to handle this event.
    
        Arguments:
            ok: True if connection up, False if failed
    
        When ok is False, this Handler is destroyed right after this method
        returns, so don't count on further communication or timeouts tied to
        this Handler. Delegate work to global Timeouts if necessary.
        """
        pass

    def shutdown_callback(self):
        """
        Called when connection is half-closed i.e. recv() returns 0
        Override if you need to know the moment it happens e.g. your
        protocol client uses shutdown() to communicate EOM.
        """
        self.destroy()

    def is_readable(self):
        return not self.connecting

    def is_writable(self):
        return self.connecting or (not not self.send_buf)

    def write_callback(self):
        self.send_callback()

    def send(self, data):
        """
        Method to append data to the output buffer, making the socket
        selectable for writing (the sending is not immediate).

        Arguments:
            data: bytes to send
        """
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
