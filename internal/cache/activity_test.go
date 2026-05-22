package cache

import "testing"

func TestListActivityItems_MentionThreadUnreadAndPriority(t *testing.T) {
	db := setupDBWithWorkspace(t)
	defer db.Close()

	const selfID = "USELF"
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	must(db.UpsertChannel(Channel{ID: "C1", WorkspaceID: "T1", Name: "general", Type: "channel", IsMember: true}))
	must(db.UpsertChannel(Channel{ID: "C2", WorkspaceID: "T1", Name: "design", Type: "channel", IsMember: true}))
	must(db.UpsertChannel(Channel{ID: "C3", WorkspaceID: "T1", Name: "ops", Type: "channel", IsMember: true}))

	must(db.UpdateChannelReadState("C1", "1700000000.000000", true))
	must(db.UpdateChannelReadState("C2", "1700000100.000000", false))
	must(db.UpdateChannelReadState("C3", "1700000200.000000", true))

	// Mention should outrank thread/unread and appear once.
	must(db.UpsertMessage(Message{TS: "1700000300.000000", ChannelID: "C1", WorkspaceID: "T1", UserID: "U2", Text: "hey <@USELF>", ThreadTS: ""}))

	// Self-authored mention should be excluded.
	must(db.UpsertMessage(Message{TS: "1700000310.000000", ChannelID: "C1", WorkspaceID: "T1", UserID: selfID, Text: "I mentioned <@USELF>", ThreadTS: ""}))

	// Thread reply.
	must(db.UpsertMessage(Message{TS: "1700000400.000000", ChannelID: "C2", WorkspaceID: "T1", UserID: "U3", Text: "parent", ThreadTS: "1700000390.000000"}))
	must(db.UpsertMessage(Message{TS: "1700000410.000000", ChannelID: "C2", WorkspaceID: "T1", UserID: "U4", Text: "reply in thread", ThreadTS: "1700000390.000000"}))
	must(db.UpsertThreadSubscription("T1", "C2", "1700000390.000000", "1700000405.000000", true))

	// Unread top-level message.
	must(db.UpsertMessage(Message{TS: "1700000500.000000", ChannelID: "C3", WorkspaceID: "T1", UserID: "U5", Text: "fresh unread", ThreadTS: ""}))

	items, err := db.ListActivityItems("T1", selfID, 20)
	if err != nil {
		t.Fatalf("ListActivityItems: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("want 3 activity items, got %d: %+v", len(items), items)
	}

	if items[0].Kind != "mention" || items[0].TS != "1700000300.000000" {
		t.Fatalf("first item = %+v, want mention at 1700000300.000000", items[0])
	}
	if items[1].Kind != "thread_reply" || items[1].TS != "1700000410.000000" {
		t.Fatalf("second item = %+v, want thread_reply at 1700000410.000000", items[1])
	}
	if items[2].Kind != "unread" || items[2].TS != "1700000500.000000" {
		t.Fatalf("third item = %+v, want unread at 1700000500.000000", items[2])
	}
}

func TestListActivityItems_DedupPrefersMention(t *testing.T) {
	db := setupDBWithWorkspace(t)
	defer db.Close()

	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.UpsertChannel(Channel{ID: "C1", WorkspaceID: "T1", Name: "general", Type: "channel", IsMember: true}))
	must(db.UpdateChannelReadState("C1", "1700000000.000000", true))
	must(db.UpsertMessage(Message{TS: "1700000600.000000", ChannelID: "C1", WorkspaceID: "T1", UserID: "U2", Text: "ping <@USELF>", ThreadTS: ""}))

	items, err := db.ListActivityItems("T1", "USELF", 20)
	if err != nil {
		t.Fatalf("ListActivityItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 deduped item, got %d: %+v", len(items), items)
	}
	if items[0].Kind != "mention" {
		t.Fatalf("kind = %q, want mention", items[0].Kind)
	}
}
