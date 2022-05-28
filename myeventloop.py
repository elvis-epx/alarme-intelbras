#!/usr/bin/env python3

import time, select, datetime
from abc import ABC, abstractmethod


LOG_ERROR = 0
LOG_WARN = 1
LOG_INFO = 2
LOG_DEBUG = 3

class Log:
    log_level = LOG_INFO

    @staticmethod
    def set_level(new_level):
        Log.log_level = new_level

    @staticmethod
    def log(level, *msg):
        if level <= Log.log_level:
            now = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
            print(now, *msg, flush=True)

    @staticmethod
    def error(*msg):
        Log.log(LOG_ERROR, *msg)

    @staticmethod
    def warn(*msg):
        Log.log(LOG_WARN, *msg)

    @staticmethod
    def info(*msg):
        Log.log(LOG_INFO, *msg)

    @staticmethod
    def debug(*msg):
        Log.log(LOG_DEBUG, *msg)


class Timeout:
    pending = {}

    @staticmethod
    def next_relative():
        absolute_to, label = Timeout.next_absolute()
        return max(0, absolute_to - time.time()), label
 
    @staticmethod
    def next_absolute():
        to = time.time() + 86400
        label = None
        for candidate in Timeout.pending.values():
            if candidate.absolute_to < to:
                to = candidate.absolute_to
                label = candidate.label
        return to, label

    @staticmethod
    def handle():
        for candidate in Timeout.pending.values():
            if time.time() > candidate.absolute_to:
                del Timeouts.pending[id(candidate)]
                candidate.callback()
                Log.debug("= timeout %s" % candidate.label)
                return True
        return False

    @staticmethod
    def _cancel(timeout_object):
        if id(timeout_object) in Timeout.pending:
            del Timeout.pending[id(timeout_object)]
            remaining_time = timeout_object.absolute_to - time.time()
            Log.debug("- timeout %s (remaining %f)" % (nome, remaining_time))

    @staticmethod
    def cancel_by_owner(owner):
        for candidate in list(Timeout.pending.values()):
            if owner is candidate.owner:
                del Timeout.pending[id(candidate)]

    def __init__(self, owner, label, relative_to, callback):
        self.owner = owner
        self.label = label
        self.relative_to = relative_to
        self.callback = callback
        self.absolute_to = time.time() + relative_to
        Log.debug("+ timeout %s %f" % (label, relative_to))

    def cancel(self):
        Timeout._cancel(self)


class Handler(ABC):
    items = {}

    @staticmethod
    def readable_fds():
        fds = []
        for handler in Handler.items.values():
            if handler.is_readable():
                fds.append(handler.fd)
        return fds 

    def is_readable(self):
        return True

    @abstractmethod
    def read_callback(self):
        pass

    @staticmethod
    def writable_fds():
        fds = []
        for handler in Handler.items.values():
            if handler.is_writable():
                fds.append(handler.fd)
        return fds 

    def is_writable(self):
        return False

    def write_callback(self):
        pass

    @staticmethod
    def exception_fds():
        fds = []
        for handler in Handler.items.values():
            if handler.is_exceptionable():
                fds.append(handler.fd)
        return fds 

    def is_exceptionable(self):
        return True

    def exception_callback(self):
        self.destroy()

    @staticmethod
    def find_by_fd(fd):
        for handler in Handler.items.values():
            if fd is handler.fd:
                return handler
        return None

    def __init__(self, label, fd, fd_exceptions):
        self.label = label
        self.fd = fd
        self.fd_exceptions = fd_exceptions
        Handler.items[id(self)] = self

    def destroy(self):
        self.log_debug("destroyed")
        del Handler.items[id(self)]
        Timeout.cancel_by_owner(self)
        try:
            self.fd.close()
        except self.fd_exceptions:
            pass

    def log_error(self, *msg):
        Log.error(self.label, *msg)

    def log_warn(self, *msg):
        Log.warn(self.label, *msg)

    def log_info(self, *msg):
        Log.info(self.label, *msg)

    def log_debug(self, *msg):
        Log.debug(self.label, *msg)


class EventLoop:
    def __init__(self):
        pass

    def loop(self):
        while self.cycle():
            pass
        Log.warn("Exiting")

    def pre_cycle(self, crd, cwr, cex, next_to, to_label):
        if to_label:
            Log.debug("Next timeout %f %s" % (next_to, to_label))

    def cycle(self):
        crd = Handler.readable_fds()
        cwr = Handler.writable_fds()
        cex = Handler.exception_fds()
        next_to, to_label = Timeout.next_relative()
        if not crd and not cex and not to_label:
            Log.warn("No remaining tasks")
            return False

        self.pre_cycle(crd, cwr, cex, next_to, to_label)
        rd, wr, ex = select.select(crd, cwr, cex, next_to)

        if rd:
            handler = Handler.find_by_fd(rd[0])
            handler.read_callback()
        elif wr:
            handler = Handler.find_by_fd(wr[0])
            handler.write_callback()
        elif ex:
            handler = Handler.find_by_fd(ex[0])
            handler.exception_callback()
        else:
            Timeout.handle()

        return True
