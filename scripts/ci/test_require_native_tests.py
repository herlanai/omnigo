import json
import unittest
from require_native_tests import verify

class NativeEvidenceTest(unittest.TestCase):
    def events(self, *events):
        return [json.dumps(event) for event in events]
    def test_requires_actual_pass(self):
        for action in ['skip', 'fail', 'run']:
            with self.assertRaises(ValueError):
                verify(self.events({'Action': action, 'Test': 'Native'}), ['Native'])
        with self.assertRaises(ValueError):
            verify([], ['Native'])
        verify(self.events({'Action': 'pass', 'Test': 'Native'}), ['Native'])
    def test_package_failure_is_not_hidden(self):
        with self.assertRaises(ValueError):
            verify(self.events({'Action': 'pass', 'Test': 'Native'}, {'Action':'fail','Package':'host'}), ['Native'])

if __name__ == '__main__':
    unittest.main()
