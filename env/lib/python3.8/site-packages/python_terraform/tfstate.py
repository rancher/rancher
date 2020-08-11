# -*- coding: utf-8 -*-
# above is for compatibility of python2.7.11

import json
import os
import logging

log = logging.getLogger(__name__)


class Tfstate(object):
    def __init__(self, data=None):
        self.tfstate_file = None
        self.native_data = data
        if data:
            self.__dict__ = data

    @staticmethod
    def load_file(file_path):
        """
        Read the tfstate file and load its contents, parses then as JSON and put the result into the object
        """
        log.debug('read data from {0}'.format(file_path))
        if os.path.exists(file_path):
            with open(file_path) as f:
                json_data = json.load(f)

            tf_state = Tfstate(json_data)
            tf_state.tfstate_file = file_path
            return tf_state

        log.debug('{0} is not exist'.format(file_path))

        return Tfstate()