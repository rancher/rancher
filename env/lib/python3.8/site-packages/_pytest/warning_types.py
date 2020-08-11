class PytestWarning(UserWarning):
    """
    Bases: :class:`UserWarning`.

    Base class for all warnings emitted by pytest.
    """


class PytestDeprecationWarning(PytestWarning, DeprecationWarning):
    """
    Bases: :class:`pytest.PytestWarning`, :class:`DeprecationWarning`.

    Warning class for features that will be removed in a future version.
    """


class RemovedInPytest4Warning(PytestDeprecationWarning):
    """
    Bases: :class:`pytest.PytestDeprecationWarning`.

    Warning class for features scheduled to be removed in pytest 4.0.
    """


class PytestExperimentalApiWarning(PytestWarning, FutureWarning):
    """
    Bases: :class:`pytest.PytestWarning`, :class:`FutureWarning`.

    Warning category used to denote experiments in pytest. Use sparingly as the API might change or even be
    removed completely in future version
    """

    @classmethod
    def simple(cls, apiname):
        return cls(
            "{apiname} is an experimental api that may change over time".format(
                apiname=apiname
            )
        )


PYTESTER_COPY_EXAMPLE = PytestExperimentalApiWarning.simple("testdir.copy_example")
