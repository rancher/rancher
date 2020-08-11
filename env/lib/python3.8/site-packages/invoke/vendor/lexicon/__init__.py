from .attribute_dict import AttributeDict
from .alias_dict import AliasDict


class Lexicon(AttributeDict, AliasDict):
    def __init__(self, *args, **kwargs):
        # Need to avoid combining AliasDict's initial attribute write on
        # self.aliases, with AttributeDict's __setattr__. Doing so results in
        # an infinite loop. Instead, just skip straight to dict() for both
        # explicitly (i.e. we override AliasDict.__init__ instead of extending
        # it.)
        # NOTE: could tickle AttributeDict.__init__ instead, in case it ever
        # grows one.
        dict.__init__(self, *args, **kwargs)
        dict.__setattr__(self, 'aliases', {})

    def __getattr__(self, key):
        # Intercept deepcopy/etc driven access to self.aliases when not
        # actually set. (Only a problem for us, due to abovementioned combo of
        # Alias and Attribute Dicts, so not solvable in a parent alone.)
        if key == 'aliases' and key not in self.__dict__:
            self.__dict__[key] = {}
        return super(Lexicon, self).__getattr__(key)
