import io
import sys
import unittest
from pathlib import Path
from unittest import TestCase

sys.path.insert(0, str(Path(__file__).resolve().parent))

from zimage_demo_progress import DemoProgress, TOTAL_BLOCKS, completed_blocks, stage_label


class DemoProgressTests(TestCase):
    def test_progress_covers_all_nine_evaluations(self):
        self.assertEqual(completed_blocks(1, 0), 1)
        self.assertEqual(completed_blocks(9, 33), TOTAL_BLOCKS)

    def test_stage_labels(self):
        self.assertEqual(stage_label(0), ("Noise Refiner", None))
        self.assertEqual(stage_label(2), ("Context Refiner", None))
        self.assertEqual(stage_label(21), ("Main Transformer", "18 / 30"))

    def test_completion_reports_hash_match_and_mismatch(self):
        matched = io.StringIO()
        display = DemoProgress("Test GPU — 8 GiB", "Prefetch", 1_005_407_748, ansi=False, stream=matched)
        display.complete(Path("canonical.png"), "7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613", 12.3, 1_005_407_748)
        self.assertIn("COMPLETE", matched.getvalue())
        self.assertIn("Canonical    MATCH", matched.getvalue())
        mismatch = io.StringIO()
        DemoProgress("Test GPU — 8 GiB", "Prefetch", 1_005_407_748, ansi=False, stream=mismatch).complete(Path("other.png"), "bad", 12.3, None)
        self.assertIn("HASH MISMATCH", mismatch.getvalue())
        self.assertIn("Canonical    MISMATCH", mismatch.getvalue())

    def test_ansi_and_plain_rendering(self):
        ansi = io.StringIO()
        DemoProgress("GPU", "Prefetch", 1, ansi=True, stream=ansi).render()
        self.assertIn("\x1b[2J", ansi.getvalue())
        plain = io.StringIO()
        DemoProgress("GPU", "Prefetch", 1, ansi=False, stream=plain).render()
        self.assertNotIn("\x1b[", plain.getvalue())


if __name__ == "__main__":
    unittest.main()
