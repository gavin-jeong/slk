package activityview

import (
	"strings"
	"testing"

	"github.com/gammons/slk/internal/cache"
)

func TestViewEmptyMentionsNoActivity(t *testing.T) {
	m := New(nil, "")
	out := m.View(5, 30)
	if !strings.Contains(strings.ToLower(out), "no activity") {
		t.Fatalf("empty view should mention no activity, got:\n%s", out)
	}
}

func TestClickAndUnreadCount(t *testing.T) {
	m := New(nil, "")
	m.SetItems([]cache.ActivityItem{
		{Kind: "mention", ChannelID: "C1", TS: "1.0", Text: "a", Unread: true},
		{Kind: "unread", ChannelID: "C2", TS: "2.0", Text: "b", Unread: false},
	})
	if m.UnreadCount() != 1 {
		t.Fatalf("UnreadCount = %d, want 1", m.UnreadCount())
	}
	if !m.ClickAt(4) {
		t.Fatal("expected click on second card row to select an item")
	}
	if got := m.SelectedIndex(); got != 1 {
		t.Fatalf("SelectedIndex = %d, want 1", got)
	}
}
