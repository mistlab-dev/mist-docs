package handler

import "testing"

// TestBatchExportConcurrencyGate: only a couple of batch exports run at a
// time. The gate is global on purpose — a batch export is both CPU- and
// memory-hungry, and the production box has 7.5GB of RAM with no swap.
func TestBatchExportConcurrencyGate(t *testing.T) {
	// Take every slot.
	for i := 0; i < cap(batchExportSlots); i++ {
		if !acquireBatchExportSlot() {
			t.Fatalf("slot %d should have been free", i)
		}
	}
	// The next one must be refused.
	if acquireBatchExportSlot() {
		t.Error("the gate should refuse a request when every slot is taken")
		releaseBatchExportSlot()
	}
	// Give the slots back.
	for i := 0; i < cap(batchExportSlots); i++ {
		releaseBatchExportSlot()
	}
	// Now a slot is free again.
	if !acquireBatchExportSlot() {
		t.Error("the gate should hand out a slot after a release")
	}
	releaseBatchExportSlot()
}

// TestUniqueBatchName: two documents with the same title must not produce
// the same zip entry name — an ambiguous archive is a problem to extract.
func TestUniqueBatchName(t *testing.T) {
	used := map[string]bool{}
	if got := uniqueBatchName(used, "报告", ".docx"); got != "报告.docx" {
		t.Errorf("first name = %q, want 报告.docx", got)
	}
	if got := uniqueBatchName(used, "报告", ".docx"); got != "报告-2.docx" {
		t.Errorf("second name = %q, want 报告-2.docx", got)
	}
	if got := uniqueBatchName(used, "报告", ".docx"); got != "报告-3.docx" {
		t.Errorf("third name = %q, want 报告-3.docx", got)
	}
}
