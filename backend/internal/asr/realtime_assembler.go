package asr

import (
	"strings"
	"unicode"
)

type RealtimeUpdate struct {
	FullText      string `json:"fullText"`
	CommittedText string `json:"committedText"`
	LiveText      string `json:"liveText"`
	DisplayText   string `json:"displayText"`
	Revision      int    `json:"revision"`
	ResultStatus  string `json:"resultStatus"`
	SegmentID     int    `json:"segmentId"`
	SegmentText   string `json:"segmentText"`
	PGS           string `json:"pgs,omitempty"`
	RG            []int  `json:"rg,omitempty"`
	BG            *int   `json:"bg,omitempty"`
	ED            *int   `json:"ed,omitempty"`
	FinalPacket   bool   `json:"finalPacket"`
}

type Packet struct {
	SegmentID   int
	Text        string
	PGS         string
	RG          []int
	BG          *int
	ED          *int
	FinalPacket bool
}

type RealtimeAssembler struct {
	segments      map[int]*segmentState
	order         []int
	fallbackSN    int
	revision      int
	lastFullText  string
	lastSegmentID int
}

type segmentState struct {
	id        int
	text      string
	finalized bool
	bg        *int
	ed        *int
}

func NewRealtimeAssembler() *RealtimeAssembler {
	return &RealtimeAssembler{
		segments: map[int]*segmentState{},
		order:    []int{},
	}
}

func (a *RealtimeAssembler) Apply(packet Packet) (RealtimeUpdate, bool) {
	if a == nil {
		return RealtimeUpdate{}, false
	}
	text := strings.TrimSpace(packet.Text)
	if text == "" {
		return RealtimeUpdate{}, false
	}
	segmentID := packet.SegmentID
	if segmentID <= 0 {
		a.fallbackSN++
		segmentID = a.fallbackSN
	}
	a.lastSegmentID = segmentID

	// 讯飞实时识别会多次修订同一段文本。带 PGS 的包按替换区间处理；
	// 不带 PGS 的包按时间范围和文本相似度合并，避免“越说越重复”。
	if strings.TrimSpace(packet.PGS) == "" {
		a.applyWithoutPGS(segmentID, packet.BG, packet.ED, text, packet.FinalPacket)
	} else {
		a.applyWithPGS(segmentID, packet.PGS, packet.RG, text, packet.FinalPacket)
	}

	fullText := a.BuildSnapshot()
	if fullText == a.lastFullText && !packet.FinalPacket {
		return RealtimeUpdate{}, false
	}
	a.lastFullText = fullText
	a.revision++
	committed := a.BuildCommittedText(segmentID, packet.FinalPacket)
	live := a.BuildLiveText(committed, fullText, text, packet.FinalPacket)
	status := "partial"
	if packet.FinalPacket {
		status = "final"
	}
	return RealtimeUpdate{
		FullText:      fullText,
		CommittedText: committed,
		LiveText:      live,
		DisplayText:   fullText,
		Revision:      a.revision,
		ResultStatus:  status,
		SegmentID:     segmentID,
		SegmentText:   text,
		PGS:           packet.PGS,
		RG:            packet.RG,
		BG:            packet.BG,
		ED:            packet.ED,
		FinalPacket:   packet.FinalPacket,
	}, true
}

func (a *RealtimeAssembler) BuildSnapshot() string {
	var builder strings.Builder
	for _, id := range a.order {
		if segment := a.segments[id]; segment != nil {
			builder.WriteString(segment.text)
		}
	}
	return builder.String()
}

func (a *RealtimeAssembler) BuildCommittedText(activeSegmentID int, finalPacket bool) string {
	if finalPacket {
		return a.BuildSnapshot()
	}
	var builder strings.Builder
	for _, id := range a.order {
		segment := a.segments[id]
		if segment == nil {
			continue
		}
		if segment.id < activeSegmentID || segment.finalized {
			builder.WriteString(segment.text)
		}
	}
	return builder.String()
}

func (a *RealtimeAssembler) BuildLiveText(committedText string, displayText string, segmentText string, finalPacket bool) string {
	if finalPacket || strings.TrimSpace(displayText) == "" {
		return ""
	}
	if committedText != "" && strings.HasPrefix(displayText, committedText) {
		return strings.TrimSpace(strings.TrimPrefix(displayText, committedText))
	}
	if strings.TrimSpace(segmentText) != "" {
		return strings.TrimSpace(segmentText)
	}
	return strings.TrimSpace(displayText)
}

func (a *RealtimeAssembler) applyWithPGS(segmentID int, pgs string, rg []int, text string, finalized bool) {
	if strings.EqualFold(pgs, "rpl") && len(rg) >= 2 {
		start, end := rg[0], rg[1]
		if start > end {
			start, end = end, start
		}
		for id := start; id <= end; id++ {
			a.remove(id)
		}
	}
	a.upsert(segmentID, text, finalized, nil, nil)
}

func (a *RealtimeAssembler) applyWithoutPGS(segmentID int, bg *int, ed *int, text string, finalized bool) {
	if bg == nil || ed == nil {
		a.upsert(segmentID, text, finalized, nil, nil)
		return
	}
	if same := a.findExactRange(*bg, *ed); same != nil && isPunctuationOnly(text) && same.text != "" {
		same.text = appendTrailingPunctuation(same.text, text)
		same.finalized = same.finalized || finalized
		return
	}
	if reusable := a.findReusableRange(*bg, *ed, text); reusable != nil {
		reusable.text = text
		reusable.finalized = reusable.finalized || finalized
		reusable.bg = copyInt(bg)
		reusable.ed = copyInt(ed)
		a.removeCoveredSiblingRanges(reusable.id, *bg, *ed, text)
		return
	}
	a.removeCoveredSiblingRanges(0, *bg, *ed, text)
	a.upsert(segmentID, text, finalized, bg, ed)
}

func (a *RealtimeAssembler) upsert(id int, text string, finalized bool, bg *int, ed *int) {
	segment := a.segments[id]
	if segment == nil {
		segment = &segmentState{id: id}
		a.segments[id] = segment
		a.order = append(a.order, id)
		sortInts(a.order)
	}
	segment.text = text
	segment.finalized = segment.finalized || finalized
	segment.bg = copyInt(bg)
	segment.ed = copyInt(ed)
}

func (a *RealtimeAssembler) remove(id int) {
	delete(a.segments, id)
	for index, value := range a.order {
		if value == id {
			a.order = append(a.order[:index], a.order[index+1:]...)
			return
		}
	}
}

func (a *RealtimeAssembler) findExactRange(bg int, ed int) *segmentState {
	for _, segment := range a.segments {
		if segment.bg != nil && segment.ed != nil && *segment.bg == bg && *segment.ed == ed {
			return segment
		}
	}
	return nil
}

func (a *RealtimeAssembler) findReusableRange(bg int, ed int, text string) *segmentState {
	var best *segmentState
	bestScore := -1.0
	for _, segment := range a.segments {
		if segment.bg == nil || segment.ed == nil || !isRangeOverlapping(bg, ed, *segment.bg, *segment.ed) {
			continue
		}
		if !isLikelySameSegmentEvolution(segment.text, text) {
			continue
		}
		score := overlapRatio(bg, ed, *segment.bg, *segment.ed)
		if containsComparableText(text, segment.text) {
			score += 1
		}
		if score > bestScore {
			bestScore = score
			best = segment
		}
	}
	return best
}

func (a *RealtimeAssembler) removeCoveredSiblingRanges(retainedID int, bg int, ed int, text string) {
	for _, id := range append([]int(nil), a.order...) {
		segment := a.segments[id]
		if segment == nil || segment.bg == nil || segment.ed == nil || id == retainedID {
			continue
		}
		if bg <= *segment.bg && ed >= *segment.ed && isLikelySameSegmentEvolution(segment.text, text) {
			a.remove(id)
		}
	}
}

func sortInts(values []int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func isRangeOverlapping(bg1 int, ed1 int, bg2 int, ed2 int) bool {
	return bg1 <= ed2 && bg2 <= ed1
}

func overlapRatio(bg1 int, ed1 int, bg2 int, ed2 int) float64 {
	start := maxInt(bg1, bg2)
	end := minInt(ed1, ed2)
	overlap := end - start
	if overlap <= 0 {
		return 0
	}
	span1 := maxInt(1, ed1-bg1)
	span2 := maxInt(1, ed2-bg2)
	return float64(overlap) / float64(minInt(span1, span2))
}

func isPunctuationOnly(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			return false
		}
	}
	return true
}

func appendTrailingPunctuation(base string, punctuation string) string {
	base = strings.TrimRightFunc(base, unicode.IsSpace)
	suffix := strings.TrimRightFunc(punctuation, unicode.IsSpace)
	if strings.HasSuffix(base, suffix) {
		return base
	}
	return base + suffix
}

func isLikelySameSegmentEvolution(existing string, incoming string) bool {
	left := comparableText(existing)
	right := comparableText(incoming)
	if left == "" || right == "" {
		return false
	}
	if left == right || strings.Contains(left, right) || strings.Contains(right, left) {
		return true
	}
	limit := minInt(len([]rune(left)), len([]rune(right)))
	if limit == 0 {
		return false
	}
	return float64(commonPrefixRunes(left, right))/float64(limit) >= 0.8
}

func containsComparableText(source string, candidate string) bool {
	sourceComparable := comparableText(source)
	candidateComparable := comparableText(candidate)
	return sourceComparable != "" && candidateComparable != "" && strings.Contains(sourceComparable, candidateComparable)
}

func comparableText(text string) string {
	var builder strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func commonPrefixRunes(left string, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	limit := minInt(len(leftRunes), len(rightRunes))
	index := 0
	for index < limit && leftRunes[index] == rightRunes[index] {
		index++
	}
	return index
}

func copyInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
