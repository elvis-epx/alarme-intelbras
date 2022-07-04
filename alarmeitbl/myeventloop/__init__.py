#!/usr/bin/env python3

import time, select, datetime, os, signal
from abc import ABC, abstractmethod

class Log:
    """
    Utility class for logging.
    """

    ERROR = 0
    WARN = 1
    INFO = 2
    DEBUG = 3
    DEBUG2 = 4

    log_level = DEBUG2
    logfile = "None"
    is_daemon = False

    mail_level = INFO
    mail_from = "None"
    mail_to = "None"

    @staticmethod
    def set_level(new_level):
        """
        Set logging level for stdout and file
        Arguments:
            new_level: Log.{ERROR,WARN,INFO,DEBUG,DEBUG2}
        """
        Log.log_level = new_level

    @staticmethod
    def set_mail(mail_level, mail_from, mail_to):
        """
        Set logging level for e-mail
        Arguments:
            mail_level: Log.{ERROR,WARN,INFO,DEBUG,DEBUG2}
            mail_from:  Sender address
            mail_to:    Recipient address
        """
        Log.mail_level = mail_level
        Log.mail_from = mail_from
        Log.mail_to = mail_to

    @staticmethod
    def set_file(logfile):
        """
        Set file name for log writing.
        By default, does not write logs to file.
        Arguments:
            logfile: The file name, or "None" (as string) if none.
        """
        Log.logfile = logfile

    @staticmethod
    def daemonize():
        """
        Indicates the logs should not be sent to stdout.
        Typically called by background() global method.
        """
        Log.is_daemon = True

    @staticmethod
    def log(level, *msg):
        now = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        msgw = now
        for item in msg:
            msgw += " "
            msgw += str(item)

        if level <= Log.log_level:
            if not Log.is_daemon:
                print(msgw, flush=True)

            if Log.logfile != "None":
                f = open(Log.logfile, "a")
                f.write(msgw)
                f.write("\r\n")
                f.close()

        if level <= Log.mail_level and Log.mail_from != 'None' and Log.mail_to != 'None':
            Log.mail(msgw)

    @staticmethod
    def mail(msg):
        # credit: http://www.thinkspot.net/sheila/article.php?story=20040822174141155
        mailbody = "From: %s\r\nTo: %s\r\nSubject: vmonitor\r\n\r\n%s\r\n" % \
                        (Log.mail_from, Log.mail_to, msg);
        MAIL = "/usr/sbin/sendmail"
        p = os.popen("%s -t" % MAIL, 'w')
        p.write(mailbody)
        exitcode = p.close()

    @staticmethod
    def error(*msg):
        """
        Logs error message
        Arguments:
            msg: list of strings, or objects convertable to strings
        """
        Log.log(Log.ERROR, *msg)

    @staticmethod
    def warn(*msg):
        """
        Logs warn message.
        Within a Handler, use Handler.log_warn() instead.
        Arguments:
            msg: list of strings, or objects convertable to strings
        """
        Log.log(Log.WARN, *msg)

    @staticmethod
    def info(*msg):
        """
        Logs info message.
        Within a Handler, use Handler.log_info() instead.
        Arguments:
            msg: list of strings, or objects convertable to strings
        """
        Log.log(Log.INFO, *msg)

    @staticmethod
    def debug(*msg):
        """
        Logs debug message.
        Within a Handler, use Handler.log_debug() instead.
        Arguments:
            msg: list of strings, or objects convertable to strings
        """
        Log.log(Log.DEBUG, *msg)

    @staticmethod
    def debug2(*msg):
        """
        Logs framework debug message.
        Within a Handler, use Handler.log_debug2() instead.
        Arguments:
            msg: list of strings, or objects convertable to strings
        """
        Log.log(Log.DEBUG2, *msg)


# background() credits: http://www.noah.org/python/daemonize.py

def background():
    """
    Puts the program in background as a daemon (service).
    """
    try:
        pid = os.fork()
        if pid > 0:
            sys.exit(0)   # Exit first parent.
    except OSError as e:
        sys.stderr.write("fork #1 failed: (%d) %s\n" % (e.errno, e.strerror))
        sys.exit(1)

    # Decouple from parent environment.
    os.chdir("/")
    os.umask(0)
    os.setsid()

    # Do second fork.
    try:
        pid = os.fork()
        if pid > 0:
            sys.exit(0)
        Log.daemonize()
    except OSError as e:
        sys.stderr.write("fork #2 failed: (%d) %s\n" % (e.errno, e.strerror))
        sys.exit(1)


class Timeout:
    """
    Class that encapsulates deferred tasks, to be run later by the event loop.
    It is generally better not to instantiate it directly, but by using
    Timeout.new(), or Handler.timeout() if within a Handler.
    """
    pending = {}

    @staticmethod
    def _next():
        # Returns next timeout to be run
        to = time.time() + 86400
        chosen = None
        for candidate in Timeout.pending.values():
            if candidate.absolute_to < to:
                to = candidate.absolute_to
                chosen = candidate
        return to, chosen

    @staticmethod
    def next_absolute():
        # Returns next timeout to be run, in absolute UNIX time.
        to, chosen = Timeout._next()
        if chosen:
            chosen = chosen.label
        return to, chosen

    @staticmethod
    def next_relative():
        # Returns next timeout to be run, in relative time.
        absolute_to, chosen = Timeout.next_absolute()
        return max(0, absolute_to - time.time()), chosen
 
    @staticmethod
    def handle():
        # Run next due timeout, calling the task back, and cancel it
        to, chosen = Timeout._next()
        if not chosen or to > time.time():
            return False

        del Timeout.pending[id(chosen)]
        Log.debug2("= timeout %s" % chosen.label)
        chosen.callback(chosen)
        return True

    @staticmethod
    def cancel_and_inval_by_owner(owner):
        # Cancel and invalidate all timeouts for a given owner (typically a Handler)
        for candidate in list(Timeout.pending.values()):
            if owner is candidate.owner:
                candidate.invalidate()
                del Timeout.pending[id(candidate)]

    @staticmethod
    def new(label, relative_to, callback):
        """
        Instantiates a new global Timeout, without an owner.
        Arguments:
            label: human-readable name of the timeout
            relative_to: relative timeout, in seconds from now
            callback: the task to be called back
        """
        return Timeout(None, label, relative_to, callback)

    def __init__(self, owner, label, relative_to, callback):
        # Not to be used directly
        self.owner = owner
        self.label = label
        self.relative_to = relative_to
        self.callback = callback
        self.invalidated = False
        self._restart()
        Log.debug2("+ timeout %s %f" % (self.label, self.relative_to))

    def invalidate(self):
        """
        Invalidate a timeout, marking it unsuitable for any future use.
        If client code tries to do anything, a exception will be raised.
        Useful for assertion/debugging.

        Called automatically when a Handler is destroyed and all timeouts
        owned by the handler are cancelled.
        """
        if self.invalidated:
            raise Exception("called Timeout.invalidate() twice")
        self.invalidated = True

    def remaining(self):
        """
        Returns time in seconds until due to be run.
        Note the timeout may have been cancelled, and this method does not
        take this into consideration.
        """
        if self.invalidated:
            raise Exception("called Timeout.remaining() on invalidated")
        return max(0, self.absolute_to - time.time())

    def _restart(self):
        if self.invalidated:
            raise Exception("called Timeout._restart() on invalidated")
        self.absolute_to = time.time() + self.relative_to
        Timeout.pending[id(self)] = self

    def restart(self):
        """
        Restart timeout with the same original relative time.
        If timeout is alive, postpones it.
        If timeout is spent/cancelled, it is rearmed.
        """
        if self.invalidated:
            raise Exception("called Timeout.restart() on invalidated")
        self._restart()
        Log.debug2("> timeout %s %f" % (self.label, self.relative_to))

    def reset(self, relative_to):
        """
        Restart timeout with a different time.
        If timeout is alive, it is cancelled and restarted.
        If timeout is spent/cancelled, it is rearmed.
        Arguments:
            relative_to: time until run the callback.
        """
        if self.invalidated:
            raise Exception("called Timeout.reset() on invalidated")
        self.relative_to = relative_to
        self.restart()

    def cancel(self):
        """
        Cancel the timeout. It can still be restarted.
        """
        if self.invalidated:
            raise Exception("called Timeout.cancel() on invalidated")
        if not self.alive():
            return False
        remaining_time = self.absolute_to - time.time()
        Log.debug2("- timeout %s (remaining %f)" % (self.label, remaining_time))
        del Timeout.pending[id(self)]
        return True

    def alive(self):
        """
        Returns whether the timeout is active.
        """
        if self.invalidated:
            raise Exception("called Timeout.alive() on invalidated")
        return id(self) in Timeout.pending


class Handler(ABC):
    """
    Abstract class that encapsulates a file descriptor.
    Create a derived class and fill in your implementation.
    """

    items = {}

    @staticmethod
    def readable_fds():
        fds = []
        for handler in Handler.items.values():
            if handler.is_readable():
                fds.append(handler.fd)
        return fds 

    def is_readable(self):
        """
        Indicates whether the file descriptor should be selected for reading.
        The base implementation always returns True.

        Override it if you need to return False in any situation.
        Do not override if you use one of the ready-made handlers for
        TCP and UDP supplied by this framework.
        """
        return True

    @abstractmethod
    def read_callback(self):
        """
        Called back by event loop when the file descriptor is ready for reading.
        You must override this method and receive the data.

        If you inherit your handler from TCPServerHandler, TCPClientHandler or
        UDPServerHandler, you should override recv_callback() instead. See the
        documentation of these classes.
        """
        pass

    @staticmethod
    def writable_fds():
        fds = []
        for handler in Handler.items.values():
            if handler.is_writable():
                fds.append(handler.fd)
        return fds 

    def is_writable(self):
        """
        Indicates whether the file descriptor should be selected for
        non-blocking writing. The base implementation always returns False.

        Override it if you want to use non-blocking writes, meaning you need
        to return True when apropriate.
        Do not override if you use one of the ready-made handlers for
        TCP and UDP supplied by this framework.
        """
        return False

    def write_callback(self):
        """
        Called back by event loop when the file descriptor is ready for writing.
        Override this if you want to do non-blocking writes.

        If you inherit your handler from TCPServerHandler, TCPClientHandler or
        UDPServerHandler, you should override send_callback() instead. See the
        documentation of these classes.
        """
        pass

    @staticmethod
    def exceptional_fds():
        fds = []
        for handler in Handler.items.values():
            if handler.is_exceptional():
                fds.append(handler.fd)
        return fds 

    def is_exceptional(self):
        """
        Indicates whether the file descriptor should be selected for
        non-blocking exceptionhan handling. The base implementation always
        returns False.

        Override it if you want to use non-blocking exceptional handling
        (e.g. TCP OOB), meaning you need to return True when apropriate.
        """
        return False

    # Override if you need to handle "exceptional" sockets (e.g. OOB) 
    def exceptional_callback(self):
        """
        Called back by event loop when the file descriptor is ready for 
        exceptional handling. The default implementation destroys the
        Handler.

        Override this if you want to do non-blocking exceptional handling.
        """
        self.destroy()

    @staticmethod
    def find_by_fd(fd):
        for handler in Handler.items.values():
            if fd is handler.fd:
                return handler
        return None

    def __init__(self, label, fd, fd_exceptions):
        """
        Creates a Handler that encapsulates a file descriptor.
        Arguments:
            label: name used to prefix logging
            fd: file descriptor to be encapsulated
            fd_exceptions: list of Exception classes expected for the
                           file descriptor type, that are gracefully
                           handled.
        """
        self.label = label
        self.fd = fd
        self.fd_exceptions = fd_exceptions
        self.destroyed = False
        Handler.items[id(self)] = self

    def destroy(self):
        """
        Destroys the handler and closes the file descriptor.

        Extend this method if you need to run code at destruction phase,
        and overriding destroyed_callback() does not suffice.
        """
        if self.destroyed:
            raise Exception("called Handler.destroy() twice")
        self.destroyed_callback()
        self.destroyed = True
        self.log_debug2("destroyed")
        del Handler.items[id(self)]
        Timeout.cancel_and_inval_by_owner(self)
        try:
            self.fd.close()
        except self.fd_exceptions:
            pass

    def destroyed_callback(self):
        """
        Called just before destruction. Override this method if you need to
        be called back upon handler destruction.
        """
        pass

    def log_error(self, *msg):
        """
        Logs error message, prefixing the handler label.
        """
        Log.error(self.label, *msg)

    def log_warn(self, *msg):
        """
        Logs warn message, prefixing the handler label.
        """
        Log.warn(self.label, *msg)

    def log_info(self, *msg):
        """
        Logs info message, prefixing the handler label.
        """
        Log.info(self.label, *msg)

    def log_debug(self, *msg):
        """
        Logs debug message, prefixing the handler label.
        """
        Log.debug(self.label, *msg)

    def log_debug2(self, *msg):
        """
        Logs framework debug message, prefixing the handler label.
        """
        Log.debug2(self.label, *msg)

    def timeout(self, label, relative_to, callback):
        """
        Creates a timeout owned by this Handler. Timeouts created this way
        are automatically cancelled and invalidated when the handler is
        destroyed.

        Arguments:
            label: a human-readable label for the Timeout
            relative_to: time in seconds into the future
            callback: the function to be called back after the time
        """
        if self.destroyed:
            raise Exception("called Handler.timeout() after destroy()")
        return Timeout(self, label, relative_to, callback)


class EventLoop:
    """
    Class that represents a program-wide event loop. This class may be
    extended, but it is expected that a single event loop is running at
    any given time.
    """

    def __init__(self):
        """
        Instantiates the event loop.
        """
        # typically used in TCP code to avoid unexpected signal
        # when a socket is written but was RSTed by the remote side
        signal.signal(signal.SIGPIPE, signal.SIG_IGN)

    def loop(self):
        """
        Runs forever until there are no Handlers or Timeouts pending.
        """
        while self.cycle():
            pass
        Log.debug2("Exiting")

    def started_cycle(self):
        """
        Called at the beginning of every event loop cycle.
        Override if you need to do something at this moment, or
        print some debugging message.
        """
        pass

    def before_select(self, crd, cwr, cex, next_to, to_label):
        """
        Called just before select() in every event loop.
        Override if you need to do something at this moment, like printing
        a message or changing the lists of file descriptors.
        """
        if to_label:
            Log.debug2("Next timeout %f %s" % (next_to, to_label))

    def cycle(self):
        # The main event loop cycle
        self.started_cycle()

        crd = Handler.readable_fds()
        cwr = Handler.writable_fds()
        cex = Handler.exceptional_fds()
        next_to, to_label = Timeout.next_relative()

        if not crd and not cwr and not cex and not to_label:
            Log.debug2("No remaining tasks")
            return False

        self.before_select(crd, cwr, cex, next_to, to_label)

        rd, wr, ex = select.select(crd, cwr, cex, next_to)

        if rd:
            handler = Handler.find_by_fd(rd[0])
            handler.read_callback()
        elif wr:
            handler = Handler.find_by_fd(wr[0])
            handler.write_callback()
        elif ex:
            handler = Handler.find_by_fd(ex[0])
            handler.exceptional_callback()
        else:
            Timeout.handle()

        return True
