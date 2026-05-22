package cache

import (
	"fmt"
	"sort"
)

// ActivityItem is one row in the Activity view. It is derived entirely from
// the local cache and represents a user-relevant event in a channel.
type ActivityItem struct {
	Kind        string // mention | thread_reply | unread
	ChannelID   string
	ChannelName string
	ChannelType string
	TS          string
	ThreadTS    string
	UserID      string
	Text        string
	Unread      bool
}

// ListActivityItems returns a cache-backed approximation of Slack's Activity
// view for the current user. It combines direct mentions, unread subscribed
// thread replies, and unread channel activity, then de-duplicates by message.
func (db *DB) ListActivityItems(workspaceID, selfUserID string, limit int) ([]ActivityItem, error) {
	if limit <= 0 {
		limit = 100
	}

	const q = `
SELECT
    m.ts,
    COALESCE(m.thread_ts, ''),
    m.channel_id,
    COALESCE(c.name, ''),
    COALESCE(c.type, ''),
    COALESCE(m.user_id, ''),
    COALESCE(m.text, ''),
    COALESCE(c.last_read_ts, ''),
    COALESCE(c.has_unread, 0),
    COALESCE(ts.last_read, ''),
    COALESCE(ts.active, 0)
FROM messages m
LEFT JOIN channels c
  ON c.id = m.channel_id
LEFT JOIN thread_subscriptions ts
  ON ts.workspace_id = m.workspace_id
 AND ts.channel_id = m.channel_id
 AND ts.thread_ts = m.thread_ts
WHERE m.workspace_id = ?
  AND m.is_deleted = 0
  AND (
    m.text LIKE ?
    OR c.has_unread = 1
    OR (ts.active = 1 AND m.thread_ts != '' AND m.ts != m.thread_ts)
  )
ORDER BY m.ts DESC
LIMIT ?
`

	mention := "%<@" + selfUserID + ">%"
	rows, err := db.conn.Query(q, workspaceID, mention, limit*10)
	if err != nil {
		return nil, fmt.Errorf("listing activity items: %w", err)
	}
	defer rows.Close()

	byKey := map[string]ActivityItem{}
	priority := map[string]int{"mention": 3, "thread_reply": 2, "unread": 1}

	for rows.Next() {
		var item ActivityItem
		var channelLastRead string
		var channelHasUnread int
		var threadLastRead string
		var threadActive int
		if err := rows.Scan(
			&item.TS,
			&item.ThreadTS,
			&item.ChannelID,
			&item.ChannelName,
			&item.ChannelType,
			&item.UserID,
			&item.Text,
			&channelLastRead,
			&channelHasUnread,
			&threadLastRead,
			&threadActive,
		); err != nil {
			return nil, fmt.Errorf("scanning activity row: %w", err)
		}

		if item.UserID == selfUserID {
			continue
		}

		kind := ""
		unread := false
		isMention := item.Text != "" && selfUserID != "" && containsMention(item.Text, selfUserID)
		isThreadReply := item.ThreadTS != "" && item.ThreadTS != item.TS
		isUnreadChannelMsg := channelHasUnread == 1 && item.TS > channelLastRead && (item.ThreadTS == "" || item.ThreadTS == item.TS || false)
		isUnreadThreadReply := threadActive == 1 && isThreadReply && item.TS > threadLastRead

		switch {
		case isMention:
			kind = "mention"
			unread = channelHasUnread == 1 || isUnreadThreadReply || item.TS > channelLastRead
		case isUnreadThreadReply:
			kind = "thread_reply"
			unread = true
		case isUnreadChannelMsg:
			kind = "unread"
			unread = true
		default:
			continue
		}

		item.Kind = kind
		item.Unread = unread
		if item.ThreadTS == "" {
			item.ThreadTS = item.TS
		}

		key := item.ChannelID + ":" + item.TS
		if existing, ok := byKey[key]; ok {
			if priority[item.Kind] > priority[existing.Kind] {
				byKey[key] = item
			}
			continue
		}
		byKey[key] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]ActivityItem, 0, len(byKey))
	for _, item := range byKey {
		out = append(out, item)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Unread != out[j].Unread {
			return out[i].Unread
		}
		if priority[out[i].Kind] != priority[out[j].Kind] {
			return priority[out[i].Kind] > priority[out[j].Kind]
		}
		return out[i].TS > out[j].TS
	})

	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func containsMention(text, userID string) bool {
	if text == "" || userID == "" {
		return false
	}
	return contains(text, "<@"+userID+">")
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
