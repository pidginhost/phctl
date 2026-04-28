package cmdutil

import (
	"fmt"
	"testing"
)

func TestFetchAll_SinglePage(t *testing.T) {
	results, err := FetchAll(func(page int32) ([]string, bool, error) {
		return []string{"a", "b", "c"}, false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
}

func TestFetchAll_MultiplePages(t *testing.T) {
	pages := [][]int{{1, 2}, {3, 4}, {5}}
	results, err := FetchAll(func(page int32) ([]int, bool, error) {
		idx := int(page - 1)
		if idx >= len(pages) {
			return nil, false, nil
		}
		hasNext := idx < len(pages)-1
		return pages[idx], hasNext, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("got %d results, want 5", len(results))
	}
	for i, v := range results {
		if v != i+1 {
			t.Errorf("results[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestFetchAll_EmptyResults(t *testing.T) {
	results, err := FetchAll(func(page int32) ([]string, bool, error) {
		return nil, false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0", len(results))
	}
}

func TestFetchAll_Error(t *testing.T) {
	_, err := FetchAll(func(page int32) ([]string, bool, error) {
		if page == 2 {
			return nil, false, fmt.Errorf("api error")
		}
		return []string{"a"}, true, nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchAll_RunawayPaginationCapped(t *testing.T) {
	calls := 0
	_, err := FetchAll(func(page int32) ([]string, bool, error) {
		calls++
		return []string{"x"}, true, nil
	})
	if err == nil {
		t.Fatal("expected error when fetcher never reports last page, got nil")
	}
	if calls > MaxPages {
		t.Errorf("fetcher called %d times, must stop at MaxPages=%d", calls, MaxPages)
	}
	if calls != MaxPages {
		t.Errorf("fetcher called %d times, want exactly MaxPages=%d", calls, MaxPages)
	}
}
