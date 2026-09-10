import unittest
from datetime import datetime, timezone
from decimal import Decimal

from migrate import canonical, converted


class ConversionTests(unittest.TestCase):
    def test_json_blob_and_text_match(self):
        self.assertEqual(canonical(b'{"a":false}', 'json'), {'a': False})
        self.assertEqual(canonical('{"a":false}', 'json'), {'a': False})

    def test_null_is_distinct_from_empty(self):
        self.assertIsNone(canonical(None, 'text'))
        self.assertEqual(canonical('', 'text'), '')

    def test_char_padding_is_database_semantics(self):
        self.assertEqual(canonical('token    ', 'character'), 'token')
        self.assertEqual(canonical('token    ', 'text'), 'token    ')

    def test_boolean_rejects_lossy_coercion(self):
        self.assertIs(canonical(0, 'boolean'), False)
        self.assertIs(canonical(1, 'boolean'), True)
        with self.assertRaises(ValueError):
            canonical(2, 'boolean')

    def test_integer_rejects_fraction(self):
        with self.assertRaises(ValueError):
            canonical(1.1, 'bigint')
        self.assertEqual(canonical(9007199254740993, 'bigint'), 9007199254740993)

    def test_decimal_is_exact(self):
        self.assertEqual(converted('12.340000', 'numeric'), Decimal('12.34'))
        self.assertEqual(canonical(Decimal('12.340000'), 'numeric'), '12.34')

    def test_timestamp_normalization(self):
        self.assertEqual(canonical('2026-09-10 08:00:00+08:00', 'timestamp with time zone'),
                         canonical(datetime(2026, 9, 10, tzinfo=timezone.utc), 'timestamp with time zone'))

    def test_binary_is_not_text(self):
        self.assertEqual(converted(b'\x00\xff', 'bytea'), b'\x00\xff')
        with self.assertRaises(UnicodeDecodeError):
            canonical(b'\xff', 'text')


if __name__ == '__main__':
    unittest.main()
