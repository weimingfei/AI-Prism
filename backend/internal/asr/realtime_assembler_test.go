package asr

import "testing"

func TestRealtimeAssemblerReplacesWPGSSegment(t *testing.T) {
	assembler := NewRealtimeAssembler()
	if update, ok := assembler.Apply(Packet{SegmentID: 1, Text: "我今天学习自注意力", PGS: "apd"}); !ok || update.FullText != "我今天学习自注意力" {
		t.Fatalf("unexpected first update: %#v ok=%v", update, ok)
	}
	update, ok := assembler.Apply(Packet{SegmentID: 1, Text: "我今天学习了自注意力", PGS: "rpl", RG: []int{1, 1}})
	if !ok {
		t.Fatal("expected replacement update")
	}
	if update.FullText != "我今天学习了自注意力" {
		t.Fatalf("expected replacement without duplication, got %q", update.FullText)
	}
}

func TestRealtimeAssemblerMergesStableAndLiveText(t *testing.T) {
	assembler := NewRealtimeAssembler()
	_, _ = assembler.Apply(Packet{SegmentID: 1, Text: "第一段。", PGS: "apd", FinalPacket: true})
	update, ok := assembler.Apply(Packet{SegmentID: 2, Text: "第二段", PGS: "apd"})
	if !ok {
		t.Fatal("expected second segment update")
	}
	if update.CommittedText != "第一段。" {
		t.Fatalf("unexpected committed text: %q", update.CommittedText)
	}
	if update.LiveText != "第二段" {
		t.Fatalf("unexpected live text: %q", update.LiveText)
	}
	if update.FullText != "第一段。第二段" {
		t.Fatalf("unexpected full text: %q", update.FullText)
	}
}

func TestRealtimeAssemblerReusesOverlappingRange(t *testing.T) {
	assembler := NewRealtimeAssembler()
	bg, ed := 100, 200
	_, _ = assembler.Apply(Packet{SegmentID: 1, Text: "Transformer", BG: &bg, ED: &ed})
	bg2, ed2 := 100, 230
	update, ok := assembler.Apply(Packet{SegmentID: 2, Text: "Transformer模型", BG: &bg2, ED: &ed2})
	if !ok {
		t.Fatal("expected evolved range update")
	}
	if update.FullText != "Transformer模型" {
		t.Fatalf("expected range evolution without duplicate sibling, got %q", update.FullText)
	}
}
