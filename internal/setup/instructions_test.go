package setup

import (
	"strings"
	"testing"
)

func TestAnEmptyFileBecomesJustTheBlock(t *testing.T) {
	got, err := withBlock("")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got != block() {
		t.Errorf("got:\n%s\nwant:\n%s", got, block())
	}
}

func TestAnExistingFileKeepsItsOwnTextAboveTheBlock(t *testing.T) {
	got, err := withBlock("# My rules\n\n1. Be brief.\n")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !strings.HasPrefix(got, "# My rules\n\n1. Be brief.\n\n") {
		t.Errorf("the author's text must stay at the top:\n%s", got)
	}
	if !strings.HasSuffix(got, block()) {
		t.Errorf("the block must be appended whole:\n%s", got)
	}
}

func TestInstallingTwiceIsAFixedPoint(t *testing.T) {
	once, err := withBlock("# My rules\n")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	twice, err := withBlock(once)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if once != twice {
		t.Errorf("a second setup must change nothing:\n%s\n---\n%s", once, twice)
	}
}

func TestAnOutOfDateBlockIsReplacedWhereItStands(t *testing.T) {
	stale := "# Top\n\n" + blockStart + "\nold helper text\n" + blockEnd + "\n\n# Bottom\n"
	got, err := withBlock(stale)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if strings.Contains(got, "old helper text") {
		t.Errorf("the stale body must be gone:\n%s", got)
	}
	if strings.Index(got, "# Top") > strings.Index(got, blockStart) {
		t.Errorf("the block must not migrate to the end of the file:\n%s", got)
	}
	if strings.Index(got, blockEnd) > strings.Index(got, "# Bottom") {
		t.Errorf("text below the block must stay below it:\n%s", got)
	}
	if strings.Count(got, blockStart) != 1 {
		t.Errorf("no duplicate block:\n%s", got)
	}
}

func TestRemovingLeavesTheSurroundingTextAlone(t *testing.T) {
	installed, err := withBlock("# My rules\n\n1. Be brief.\n")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := withoutBlock(installed)
	if err != nil || got == nil {
		t.Fatalf("remove: got (%v, %v)", got, err)
	}
	if *got != "# My rules\n\n1. Be brief.\n" {
		t.Errorf("got %q", *got)
	}
}

func TestRemovingKeepsTextThatCameAfterTheBlock(t *testing.T) {
	text := "# Top\n\n" + blockStart + "\nhelper\n" + blockEnd + "\n\n# Bottom\n"
	got, err := withoutBlock(text)
	if err != nil || got == nil {
		t.Fatalf("remove: got (%v, %v)", got, err)
	}
	if *got != "# Top\n\n# Bottom\n" {
		t.Errorf("got %q", *got)
	}
}

func TestAFileTheHelperNeverTouchedIsLeftUntouched(t *testing.T) {
	for _, text := range []string{"# My rules\n", ""} {
		got, err := withoutBlock(text)
		if err != nil || got != nil {
			t.Errorf("withoutBlock(%q): got (%v, %v), want (nil, nil)", text, got, err)
		}
	}
}

func TestAFileThatWasOnlyTheBlockEndsUpEmpty(t *testing.T) {
	got, err := withoutBlock(block())
	if err != nil || got == nil {
		t.Fatalf("remove: got (%v, %v)", got, err)
	}
	if *got != "" {
		t.Errorf("got %q, want an empty file the caller can take away", *got)
	}
}

func TestAHalfDeletedBlockIsRefusedRatherThanGuessedAt(t *testing.T) {
	// Either marker alone leaves no honest way to tell the helper's text from
	// the author's, so both commands stop instead of eating somebody's file.
	if _, err := withBlock("# Top\n" + blockStart + "\nhelper\n"); err == nil {
		t.Error("a missing end marker must be refused")
	}
	if _, err := withBlock("# Top\nhelper\n" + blockEnd + "\n"); err == nil {
		t.Error("a missing start marker must be refused")
	}
	if _, err := withoutBlock(blockEnd + "\n" + blockStart + "\n"); err == nil {
		t.Error("markers in the wrong order must be refused")
	}
}
