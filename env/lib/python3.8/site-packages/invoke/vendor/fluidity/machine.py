import re
import inspect
from .backwardscompat import callable

# metaclass implementation idea from
# http://blog.ianbicking.org/more-on-python-metaprogramming-comment-14.html
_transition_gatherer = []

def transition(event, from_, to, action=None, guard=None):
    _transition_gatherer.append([event, from_, to, action, guard])

_state_gatherer = []

def state(name, enter=None, exit=None):
    _state_gatherer.append([name, enter, exit])


class MetaStateMachine(type):

    def __new__(cls, name, bases, dictionary):
        global _transition_gatherer, _state_gatherer
        Machine = super(MetaStateMachine, cls).__new__(cls, name, bases, dictionary)
        Machine._class_transitions = []
        Machine._class_states = {}
        for s in _state_gatherer:
            Machine._add_class_state(*s)
        for i in _transition_gatherer:
            Machine._add_class_transition(*i)
        _transition_gatherer = []
        _state_gatherer = []
        return Machine


StateMachineBase = MetaStateMachine('StateMachineBase', (object, ), {})


class StateMachine(StateMachineBase):

    def __init__(self):
        self._bring_definitions_to_object_level()
        self._inject_into_parts()
        self._validate_machine_definitions()
        if callable(self.initial_state):
            self.initial_state = self.initial_state()
        self._current_state_object = self._state_by_name(self.initial_state)
        self._current_state_object.run_enter(self)
        self._create_state_getters()

    def __new__(cls, *args, **kwargs):
        obj = super(StateMachine, cls).__new__(cls)
        obj._states = {}
        obj._transitions = []
        return obj

    def _bring_definitions_to_object_level(self):
        self._states.update(self.__class__._class_states)
        self._transitions.extend(self.__class__._class_transitions)

    def _inject_into_parts(self):
        for collection in [self._states.values(), self._transitions]:
            for component in collection:
                component.machine = self

    def _validate_machine_definitions(self):
        if len(self._states) < 2:
            raise InvalidConfiguration('There must be at least two states')
        if not getattr(self, 'initial_state', None):
            raise InvalidConfiguration('There must exist an initial state')

    @classmethod
    def _add_class_state(cls, name, enter, exit):
        cls._class_states[name] = _State(name, enter, exit)

    def add_state(self, name, enter=None, exit=None):
        state = _State(name, enter, exit)
        setattr(self, state.getter_name(), state.getter_method().__get__(self, self.__class__))
        self._states[name] = state

    def _current_state_name(self):
        return self._current_state_object.name

    current_state = property(_current_state_name)

    def changing_state(self, from_, to):
        """
        This method is called whenever a state change is executed
        """
        pass

    def _new_state(self, state):
        self.changing_state(self._current_state_object.name, state.name)
        self._current_state_object = state

    def _state_objects(self):
        return list(self._states.values())

    def states(self):
        return [s.name for s in self._state_objects()]

    @classmethod
    def _add_class_transition(cls, event, from_, to, action, guard):
        transition = _Transition(event, [cls._class_states[s] for s in _listize(from_)],
            cls._class_states[to], action, guard)
        cls._class_transitions.append(transition)
        setattr(cls, event, transition.event_method())

    def add_transition(self, event, from_, to, action=None, guard=None):
        transition = _Transition(event, [self._state_by_name(s) for s in _listize(from_)],
            self._state_by_name(to), action, guard)
        self._transitions.append(transition)
        setattr(self, event, transition.event_method().__get__(self, self.__class__))

    def _process_transitions(self, event_name, *args, **kwargs):
        transitions = self._transitions_by_name(event_name)
        transitions = self._ensure_from_validity(transitions)
        this_transition = self._check_guards(transitions)
        this_transition.run(self, *args, **kwargs)

    def _create_state_getters(self):
        for state in self._state_objects():
            setattr(self, state.getter_name(), state.getter_method().__get__(self, self.__class__))

    def _state_by_name(self, name):
        for state in self._state_objects():
            if state.name == name:
                return state

    def _transitions_by_name(self, name):
        return list(filter(lambda transition: transition.event == name, self._transitions))

    def _ensure_from_validity(self, transitions):
        valid_transitions = list(filter(
          lambda transition: transition.is_valid_from(self._current_state_object),
          transitions))
        if len(valid_transitions) == 0:
            raise InvalidTransition("Cannot %s from %s" % (
                transitions[0].event, self.current_state))
        return valid_transitions

    def _check_guards(self, transitions):
        allowed_transitions = []
        for transition in transitions:
            if transition.check_guard(self):
                allowed_transitions.append(transition)
        if len(allowed_transitions) == 0:
            raise GuardNotSatisfied("Guard is not satisfied for this transition")
        elif len(allowed_transitions) > 1:
            raise ForkedTransition("More than one transition was allowed for this event")
        return allowed_transitions[0]


class _Transition(object):

    def __init__(self, event, from_, to, action, guard):
        self.event = event
        self.from_ = from_
        self.to = to
        self.action = action
        self.guard = _Guard(guard)

    def event_method(self):
        def generated_event(machine, *args, **kwargs):
            these_transitions = machine._process_transitions(self.event, *args, **kwargs)
        generated_event.__doc__ = 'event %s' % self.event
        generated_event.__name__ = self.event
        return generated_event

    def is_valid_from(self, from_):
        return from_ in _listize(self.from_)

    def check_guard(self, machine):
        return self.guard.check(machine)

    def run(self, machine, *args, **kwargs):
        machine._current_state_object.run_exit(machine)
        machine._new_state(self.to)
        self.to.run_enter(machine)
        _ActionRunner(machine).run(self.action, *args, **kwargs)


class _Guard(object):

    def __init__(self, action):
        self.action = action

    def check(self, machine):
        if self.action is None:
            return True
        items = _listize(self.action)
        result = True
        for item in items:
            result = result and self._evaluate(machine, item)
        return result

    def _evaluate(self, machine, item):
        if callable(item):
            return item(machine)
        else:
            guard = getattr(machine, item)
            if callable(guard):
                guard = guard()
            return guard


class _State(object):

    def __init__(self, name, enter, exit):
        self.name = name
        self.enter = enter
        self.exit = exit

    def getter_name(self):
        return 'is_%s' % self.name

    def getter_method(self):
        def state_getter(self_machine):
            return self_machine.current_state == self.name
        return state_getter

    def run_enter(self, machine):
        _ActionRunner(machine).run(self.enter)

    def run_exit(self, machine):
        _ActionRunner(machine).run(self.exit)


class _ActionRunner(object):

    def __init__(self, machine):
        self.machine = machine

    def run(self, action_param, *args, **kwargs):
        if not action_param:
            return
        action_items = _listize(action_param)
        for action_item in action_items:
            self._run_action(action_item, *args, **kwargs)

    def _run_action(self, action, *args, **kwargs):
        if callable(action):
            self._try_to_run_with_args(action, self.machine, *args, **kwargs)
        else:
            self._try_to_run_with_args(getattr(self.machine, action), *args, **kwargs)

    def _try_to_run_with_args(self, action, *args, **kwargs):
        try:
            action(*args, **kwargs)
        except TypeError:
            action()


class InvalidConfiguration(Exception):
    pass


class InvalidTransition(Exception):
    pass


class GuardNotSatisfied(Exception):
    pass


class ForkedTransition(Exception):
    pass


def _listize(value):
    return type(value) in [list, tuple] and value or [value]

