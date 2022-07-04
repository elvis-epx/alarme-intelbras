#!/usr/bin/env python3

import socket, time, datetime
from abc import ABC, abstractmethod
from . import Timeout, Handler, EventLoop

class UDPServerHandler(Handler):
    """
    Handler specialization to encapsulate and handle UDP packets.
    This is an abstract class, and there are methods you are required
    to override to complete the implementation.
    """
    def __init__(self, addr, label=None):
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEPORT, 1)
        sock.bind(addr)
        label = label or ("%s:%d" % addr)
        super().__init__(label, sock, socket.error)
        self.send_buf = []

    def read_callback(self):
        try:
            dgram, addr = self.fd.recvfrom(4096)
            self.log_debug2("Received %s" % dgram)
        except socket.error:
            self.log_warn("Error recvfrom")
            return
        self.recv_callback(addr, dgram)

    @abstractmethod
    def recv_callback(self, addr, dgram):
        """
        Override this abstract method to receive packets from UDP peers.
        This class does not buffer incoming data.

        Arguments:
            addr: tuple with address and port of the peer
            dgram: packet data
        """
        pass

    def is_writable(self):
        return not not self.send_buf

    def write_callback(self):
        self.send_callback()

    def send_callback(self):
        try:
            self.fd.sendto(self.send_buf[0]['dgram'], 0, self.send_buf[0]['addr'])
        except socket.error as err:
            self.log_warn("exception writing sk", err)
        self.send_buf = self.send_buf[1:]

    def sendto(self, addr, dgram):
        """
        Method to append packets to the output buffer, making the
        socket selectable for writing.

        Arguments:
            addr: tuple with address and port of the peer
            dgram: packet data
        """
        self.send_buf.append({'addr': addr, 'dgram': dgram})


class UDPServerEventLoop(EventLoop):
    def __init__(self):
        super().__init__()
