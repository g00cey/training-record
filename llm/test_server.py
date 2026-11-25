#!/usr/bin/env python3
"""Unit tests for Vision LLM JSON sanitization and normalization in server.py."""

import unittest

import server


class ParseVisionJsonTests(unittest.TestCase):
    def test_valid_json(self):
        raw = (
            '{"completedTime":"00:28:24","avgHeartRate":104,"maxHeartRate":136,'
            '"distanceKm":null,"hrZones":{"ウォームアップ":"00:17:30"},'
            '"freeNotes":"","uncertainFields":[]}'
        )
        parsed = server.parse_vision_json(raw)
        self.assertEqual(parsed["completedTime"], "00:28:24")
        self.assertEqual(parsed["avgHeartRate"], 104)
        self.assertEqual(parsed["uncertainFields"], [])

    def test_reported_broken_json_with_escaped_newlines(self):
        raw = (
            '{"completedTime":"00:28:24","avgHeartRate":104,"maxHeartRate":136,'
            '"distanceKm":null,"hrZones":{"ウォームアップ":"00:17:30",'
            '"インテンシブ":"00:06:08","有酸素":"00:01:25","無酸素":"00:0.oo.oo",'
            '"最大酸素摂取量":" _ _ _ _ _ \\n _ _ \\n 摂取\\n 量": " _ _ \\n  _ \\n  量": "00:00:00"},'
            '"freeNotes":"","uncertainFields[]}'
        )
        parsed = server.parse_vision_json(raw)
        normalized = server.validate_and_normalize(parsed)

        self.assertEqual(normalized["durationMinutes"], 28)
        self.assertEqual(normalized["avgHeartRate"], 104)
        self.assertEqual(normalized["maxHeartRate"], 136)
        self.assertIsNone(normalized["distanceKm"])
        self.assertEqual(normalized["hrZones"]["ウォームアップ"], "17:30")
        self.assertEqual(normalized["hrZones"]["インテンシブ"], "06:08")
        self.assertEqual(normalized["hrZones"]["有酸素"], "01:25")
        self.assertNotIn("無酸素", normalized["hrZones"])
        self.assertEqual(normalized["hrZones"]["最大酸素摂取量"], "00:00")
        self.assertIn("hrZones", normalized["uncertainFields"])

    def test_reported_broken_json_with_raw_newlines(self):
        raw = (
            '{"completedTime":"00:28:24","avgHeartRate":104,"maxHeartRate":136,'
            '"distanceKm":null,"hrZones":{"ウォームアップ":"00:17:30",'
            '"インテンシブ":"00:06:08","有酸素":"00:01:25","無酸素":"00:0.oo.oo",'
            '"最大酸素摂取量":" _ _ _ _ _ \n _ _ \n 摂取\n 量": " _ _ \n  _ \n  量": "00:00:00"},'
            '"freeNotes":"","uncertainFields[]}'
        )
        parsed = server.parse_vision_json(raw)
        self.assertEqual(parsed["hrZones"]["最大酸素摂取量"], "00:00:00")
        self.assertEqual(parsed["uncertainFields"], [])

    def test_code_fence_and_surrounding_text(self):
        raw = (
            '読み取り結果:\n```json\n'
            '{"completedTime":"00:10:00","avgHeartRate":120,"maxHeartRate":140,'
            '"distanceKm":4.2,"hrZones":{},"freeNotes":"","uncertainFields":[]}\n'
            '```\n以上です。'
        )
        parsed = server.parse_vision_json(raw)
        self.assertEqual(parsed["distanceKm"], 4.2)

    def test_fallback_extraction_on_severely_broken_json(self):
        raw = (
            '{"completedTime":"00:25:00","avgHeartRate":110,"maxHeartRate":145,'
            '"distanceKm":null,"hrZones":{"ウォームアップ":"00:05:00",'
            '"freeNotes":"途中で壊れた "引用符" あり'
        )
        parsed = server.parse_vision_json(raw)
        self.assertEqual(parsed["completedTime"], "00:25:00")
        self.assertEqual(parsed["avgHeartRate"], 110)
        self.assertEqual(parsed["maxHeartRate"], 145)
        self.assertEqual(parsed["hrZones"]["ウォームアップ"], "00:05:00")

    def test_empty_or_non_json_raises(self):
        with self.assertRaises(RuntimeError):
            server.parse_vision_json("")
        with self.assertRaises(RuntimeError):
            server.parse_vision_json("画像を読み取れませんでした。")


class ValidateAndNormalizeTests(unittest.TestCase):
    def test_invalid_completed_time_marks_duration_uncertain(self):
        normalized = server.validate_and_normalize({
            "completedTime": "00:xx:00",
            "avgHeartRate": 100,
            "maxHeartRate": 120,
            "hrZones": {},
            "uncertainFields": [],
        })
        self.assertIsNone(normalized["durationMinutes"])
        self.assertIn("durationMinutes", normalized["uncertainFields"])


if __name__ == "__main__":
    unittest.main()
