#!/usr/bin/env python3

import socket, time, datetime
from abc import ABC, abstractmethod
from . import Timeout, Handler, EventLoop

class TCPServerHandler(Handler):
    """
    Handler specialization to encapsulate and handle TCP connections.
    This is an abstract class, and there are methods you are required
    to override to complete the implementation.
    """
    def __init__(self, addr, sock):
        """
        Instantiate handler of received TCP connection.
        Generally invoked by a Handler of a listening socket.
        Arguments:
            addr: address/port tuple of the client side
            sock: the socket file descriptor of the connection
        """
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

    def shutdown_callback(self):
        """
        Called when connection is half-closed i.e. recv() returns 0
        Override if you need to know the moment it happens e.g. your
        protocol client uses shutdown() to communicate EOM.
        """
        self.destroy()

    def is_writable(self):
        return not not self.send_buf

    def send(self, data):
        """
        Method to append data to the output buffer, making the socket
        selectable for writing (the sending is not immediate).

        Arguments:
            data: bytes to send
        """
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
    """
    Handler subclass specialized in listening TCP socket.
    """

    def __init__(self, addr, handler_class):
        """
        Instantiate TCP listener socket and a Handler to encapsulate it.
        Arguments:
            addr: Tuple of network interface addr and port to listen on
            handler_class: the Handler class that will encapsulate the
                           connections stemming from this listener
                           (typically a subclass of TCPServerHandler)
        """
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
    """
    Event loop subclass, conceived to be a shorthand for TCP Servers.
    """
    # listener_class expected to be subclass of TCPListener
    # handler_class expected to be subclass of TCPServerHandler
    def __init__(self, addr, listener_class, handler_class):
        """
        Instantiates an event loop plus a TCP Server.
        Arguments:
            addr: Tuple of network interface addr and port to listen on
            listener_class: Class responsible for creating the listening
                            socket, automatically instantiated by this
                            method. An example is TCPListener, but any
                            class with the same protocol can be used.
            handler_class: Class responsible for encapsulating the sockets
                           of TCP connections stemming from the listening
                           socket. Typically a subclass of TCPServerHandler.
        """
        listener = listener_class(addr, handler_class)
        super().__init__()
