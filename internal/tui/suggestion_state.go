package tui

type suggestionState struct {
	index      int
	offset     int
	maxVisible int
}

func newSuggestionState(maxVisible int) suggestionState {
	if maxVisible <= 0 {
		maxVisible = 6
	}
	return suggestionState{index: 0, offset: 0, maxVisible: maxVisible}
}

func (state *suggestionState) normalize(total int) {
	if total <= 0 {
		state.index = 0
		state.offset = 0
		return
	}

	if state.index < 0 {
		state.index = 0
	}
	if state.index >= total {
		state.index = total - 1
	}

	if state.offset < 0 {
		state.offset = 0
	}
	maxOffset := total - state.maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if state.offset > maxOffset {
		state.offset = maxOffset
	}

	if state.index < state.offset {
		state.offset = state.index
	}
	if state.index >= state.offset+state.maxVisible {
		state.offset = state.index - state.maxVisible + 1
	}
}

func (state *suggestionState) move(step int, total int) bool {
	if total <= 0 {
		return false
	}

	state.normalize(total)
	next := state.index + step
	if next < 0 {
		next = 0
	}
	if next >= total {
		next = total - 1
	}
	state.index = next
	state.normalize(total)
	return true
}

func (state *suggestionState) visibleRange(total int) (int, int) {
	state.normalize(total)
	start := state.offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + state.maxVisible
	if end > total {
		end = total
	}
	return start, end
}
