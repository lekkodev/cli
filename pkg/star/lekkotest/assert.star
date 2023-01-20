# Predeclared built-ins for this module:
#
# error(msg): report an error in Go's test framework without halting execution.
#  This is distinct from the built-in fail function, which halts execution.
# catch(f): evaluate f() and returns its evaluation error message, if any
# matches(str, pattern): report whether str matches regular expression pattern.
# module(**kwargs): a constructor for a module.
# _freeze(x): freeze the value x and everything reachable from it.
#
# Clients may use these functions to define their own testing abstractions.

def _err(msg, extra = ''):
    if extra:
        msg += ": %s" % extra
    error(msg)

def _eq(x, y, msg = ""):
    if x != y:
        _err("%r != %r" % (x, y), msg)

def _ne(x, y):
    if x == y:
        _err("%r == %r" % (x, y), msg)

def _true(cond, msg = "assertion failed"):
    if not cond:
        _err(msg)

def _lt(x, y, msg = ""):
    if not (x < y):
        _err("%s is not less than %s" % (x, y), msg)

def _contains(x, y, msg = ""):
    if y not in x:
        _err("%s does not contain %s" % (x, y), msg)

def _fails(f, pattern, msg = ""):
    "assert_fails asserts that evaluation of f() fails with the specified error."
    msg = catch(f)
    if msg == None:
        _err("evaluation succeeded unexpectedly (want error matching %r)" % pattern, msg)
    elif not matches(pattern, msg):
        _err("regular expression (%s) did not match error (%s)" % (pattern, msg), msg)

freeze = _freeze  # an exported global whose value is the built-in freeze function

def _eq(x, y, msg = ""):
    

assert = module(
    "assert",
    fail = error,
    eq = _eq,
    ne = _ne,
    true = _true,
    lt = _lt,
    contains = _contains,
    fails = _fails,
)

lekko = module(
    "lekko",
    fail = error,
    eq = 
)