package notifications

import (
	"context"
	"errors"
	"fmt"

	"devclubxnst.com/cadence/internal/modules/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSubscriptionNotFound = errors.New("push subscription not found")
)

type PushSubscriptionRecord struct {
	Endpoint string
	P256dh   string
	Auth     string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func resolveEventID(ctx context.Context) string {
	if evCtx, ok := events.FromContext(ctx); ok && evCtx != nil && evCtx.EventID != "" {
		return evCtx.EventID
	}
	if ev, ok := events.GetEvent(ctx); ok && ev != nil && ev.ID != "" {
		return ev.ID
	}
	return events.DefaultEventID
}

// UpsertSubscription inserts or updates a client's push endpoint and cryptographic keys.
func (r *Repository) UpsertSubscription(
	ctx context.Context,
	userID, endpoint, p256dh, authKey string,
	userAgent *string,
) error {
	query := `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth,
		    user_agent = EXCLUDED.user_agent,
		    updated_at = CURRENT_TIMESTAMP;
	`
	_, err := r.db.Exec(ctx, query, userID, endpoint, p256dh, authKey, userAgent)
	if err != nil {
		return fmt.Errorf("upsert push subscription: %w", err)
	}
	return nil
}

// DeleteUserSubscription removes a push subscription endpoint belonging to the specified user.
func (r *Repository) DeleteUserSubscription(ctx context.Context, userID, endpoint string) error {
	query := `DELETE FROM push_subscriptions WHERE endpoint = $1 AND user_id = $2;`
	res, err := r.db.Exec(ctx, query, endpoint, userID)
	if err != nil {
		return fmt.Errorf("delete user push subscription: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSubscriptionNotFound
	}
	return nil
}

// DeleteSubscription removes a push subscription endpoint (used for pruning 404/410 by worker).
func (r *Repository) DeleteSubscription(ctx context.Context, endpoint string) error {
	query := `DELETE FROM push_subscriptions WHERE endpoint = $1;`
	_, err := r.db.Exec(ctx, query, endpoint)
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

// GetEligibleRecipientIDs resolves target user IDs in the active event based on broadcast target type:
// - "event" or "broadcast": all non-banned active users in the event
// - "competition": all non-banned active team members in the specified competition of this event (by competition ID or slug)
// - "team": all non-banned active members of the specified team in this event
// - "user": a single non-banned active user in this event
// - "role": all non-banned active users with the specified event role
// - "attendance_present" / "attendance_absent": attendees present/absent in the specified scan session
func (r *Repository) GetEligibleRecipientIDs(ctx context.Context, targetType string, targetID *string) ([]string, error) {
	eventID := resolveEventID(ctx)
	var query string
	var args []any

	switch targetType {
	case "event", "broadcast":
		query = `
			SELECT u.id::text
			FROM user_events ue
			JOIN users u ON u.id = ue.user_id
			WHERE ue.event_id = $1 AND ue.is_banned = FALSE AND u.deleted_at IS NULL;`
		args = append(args, eventID)
	case "competition":
		if targetID == nil || *targetID == "" {
			return nil, fmt.Errorf("competition target_id is required")
		}
		var compID string
		err := r.db.QueryRow(ctx, `
			SELECT id::text
			FROM competitions
			WHERE event_id = $1 AND (id::text = $2 OR LOWER(slug) = LOWER($2))
			LIMIT 1
		`, eventID, *targetID).Scan(&compID)
		if err != nil {
			return nil, fmt.Errorf("competition not found in this event")
		}
		query = `
			SELECT DISTINCT u.id::text
			FROM team_members tm
			JOIN users u ON u.id = tm.user_id
			LEFT JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = $2
			WHERE tm.competition_id = $1::uuid
			  AND COALESCE(ue.is_banned, FALSE) = FALSE
			  AND u.deleted_at IS NULL;`
		args = append(args, compID, eventID)
	case "team":
		if targetID == nil || *targetID == "" {
			return nil, fmt.Errorf("team target_id is required")
		}
		var exists bool
		if err := r.db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM teams t
				JOIN competitions c ON c.id = t.competition_id
				WHERE t.id::text = $1 AND c.event_id = $2
			)
		`, *targetID, eventID).Scan(&exists); err != nil || !exists {
			return nil, fmt.Errorf("team not found in this event")
		}
		query = `
			SELECT DISTINCT u.id::text
			FROM team_members tm
			JOIN teams t ON t.id = tm.team_id
			JOIN competitions c ON c.id = t.competition_id
			JOIN users u ON u.id = tm.user_id
			LEFT JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = c.event_id
			WHERE tm.team_id = $1::uuid
			  AND c.event_id = $2
			  AND COALESCE(ue.is_banned, FALSE) = FALSE
			  AND u.deleted_at IS NULL;`
		args = append(args, *targetID, eventID)
	case "role":
		if targetID == nil || *targetID == "" {
			return nil, fmt.Errorf("role target_id is required")
		}
		query = `
			SELECT u.id::text
			FROM user_events ue
			JOIN users u ON u.id = ue.user_id
			WHERE ue.event_id = $1 AND ue.role = $2::event_role AND ue.is_banned = FALSE AND u.deleted_at IS NULL;`
		args = append(args, eventID, *targetID)
	case "user":
		if targetID == nil || *targetID == "" {
			return nil, fmt.Errorf("user target_id is required")
		}
		query = `
			SELECT u.id::text
			FROM users u
			JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = $2
			WHERE u.id::text = $1 AND ue.is_banned = FALSE AND u.deleted_at IS NULL;`
		args = append(args, *targetID, eventID)
	case "attendance_present", "attendance_absent":
		if targetID == nil || *targetID == "" {
			return nil, fmt.Errorf("attendance session target_id is required")
		}
		cond := "EXISTS"
		if targetType == "attendance_absent" {
			cond = "NOT EXISTS"
		}
		query = `
			SELECT u.id::text FROM users u
			JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = $2
			LEFT JOIN tickets t ON t.user_id = u.id AND t.event_id = $2
			WHERE u.deleted_at IS NULL
			  AND ue.role = 'participant'
			  AND ue.is_banned = FALSE
			  AND EXISTS (SELECT 1 FROM scan_sessions ss WHERE ss.id::text = $1 AND ss.event_id = $2)
			  AND ` + cond + ` (SELECT 1 FROM scan_events e WHERE e.ticket_id = t.id AND e.session_id::text = $1);`
		args = append(args, *targetID, eventID)
	default:
		return nil, fmt.Errorf("unknown target type: %s", targetType)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query eligible recipients: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		userIDs = append(userIDs, uid)
	}
	return userIDs, nil
}

// CreateNotification creates the notification record and recipient entries in a transaction.
func (r *Repository) CreateNotification(
	ctx context.Context,
	title, body string,
	actionURL, iconURL *string,
	targetType string,
	targetID *string,
	createdBy *string,
	recipientIDs []string,
) (string, error) {
	eventID := resolveEventID(ctx)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var notifID string
	insertNotifQuery := `
		INSERT INTO notifications (event_id, title, body, action_url, icon_url, target_type, target_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id;
	`
	err = tx.QueryRow(ctx, insertNotifQuery, eventID, title, body, actionURL, iconURL, targetType, targetID, createdBy).Scan(&notifID)
	if err != nil {
		return "", fmt.Errorf("insert notification: %w", err)
	}

	if len(recipientIDs) > 0 {
		insertRecipientsQuery := `
			INSERT INTO notification_recipients (notification_id, user_id)
			SELECT $1, unnest($2::uuid[])
			ON CONFLICT DO NOTHING;
		`
		if _, err := tx.Exec(ctx, insertRecipientsQuery, notifID, recipientIDs); err != nil {
			return "", fmt.Errorf("insert notification recipients: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}

	return notifID, nil
}

// GetPushSubscriptionsForUsers retrieves active push subscriptions for a list of target user IDs.
func (r *Repository) GetPushSubscriptionsForUsers(ctx context.Context, userIDs []string) ([]PushSubscriptionRecord, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT endpoint, p256dh, auth
		FROM push_subscriptions
		WHERE user_id = ANY($1::uuid[]);
	`
	rows, err := r.db.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query push subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []PushSubscriptionRecord
	for rows.Next() {
		var s PushSubscriptionRecord
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, nil
}

// GetUserInbox fetches paginated inbox items for the user in the active event, plus total and unread counts.
func (r *Repository) GetUserInbox(ctx context.Context, userID string, page, limit int) (*InboxResponse, error) {
	eventID := resolveEventID(ctx)
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var unreadCount int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notification_recipients nr
		JOIN notifications n ON n.id = nr.notification_id
		WHERE nr.user_id = $1 AND n.event_id = $2 AND nr.is_read = false;`, userID, eventID).Scan(&unreadCount)
	if err != nil {
		return nil, fmt.Errorf("count unread: %w", err)
	}

	var totalCount int
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notification_recipients nr
		JOIN notifications n ON n.id = nr.notification_id
		WHERE nr.user_id = $1 AND n.event_id = $2;`, userID, eventID).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("count total: %w", err)
	}

	query := `
		SELECT n.id, n.title, n.body, n.action_url, n.icon_url, nr.is_read, nr.read_at, n.created_at
		FROM notification_recipients nr
		JOIN notifications n ON n.id = nr.notification_id
		WHERE nr.user_id = $1 AND n.event_id = $2
		ORDER BY n.created_at DESC
		LIMIT $3 OFFSET $4;
	`
	rows, err := r.db.Query(ctx, query, userID, eventID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query inbox items: %w", err)
	}
	defer rows.Close()

	var items []NotificationItem
	for rows.Next() {
		var item NotificationItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Body, &item.ActionURL, &item.IconURL, &item.IsRead, &item.ReadAt, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification item: %w", err)
		}
		items = append(items, item)
	}

	if items == nil {
		items = []NotificationItem{}
	}

	return &InboxResponse{
		Notifications: items,
		UnreadCount:   unreadCount,
		TotalCount:    totalCount,
		Page:          page,
		Limit:         limit,
	}, nil
}

// MarkAsRead marks a single notification as read for the user.
func (r *Repository) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	query := `
		UPDATE notification_recipients
		SET is_read = true, read_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND notification_id = $2;
	`
	_, err := r.db.Exec(ctx, query, userID, notificationID)
	if err != nil {
		return fmt.Errorf("mark notification as read: %w", err)
	}
	return nil
}

// MarkAllAsRead marks all notifications in the active event as read for the user.
func (r *Repository) MarkAllAsRead(ctx context.Context, userID string) error {
	eventID := resolveEventID(ctx)
	query := `
		UPDATE notification_recipients nr
		SET is_read = true, read_at = CURRENT_TIMESTAMP
		FROM notifications n
		WHERE nr.notification_id = n.id AND nr.user_id = $1 AND n.event_id = $2 AND nr.is_read = false;
	`
	_, err := r.db.Exec(ctx, query, userID, eventID)
	if err != nil {
		return fmt.Errorf("mark all notifications as read: %w", err)
	}
	return nil
}

// GetAdminHistory retrieves sent notifications in the active event with recipient and read counts.
func (r *Repository) GetAdminHistory(ctx context.Context, page, limit int) (*AdminHistoryResponse, error) {
	eventID := resolveEventID(ctx)
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var totalCount int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE event_id = $1;`, eventID).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("count notifications: %w", err)
	}

	query := `
		SELECT 
			n.id, n.title, n.body, n.action_url, n.target_type, n.target_id,
			u.email,
			COUNT(nr.user_id) as recipients_count,
			COUNT(CASE WHEN nr.is_read THEN 1 END) as read_count,
			n.created_at
		FROM notifications n
		LEFT JOIN users u ON u.id = n.created_by
		LEFT JOIN notification_recipients nr ON nr.notification_id = n.id
		WHERE n.event_id = $1
		GROUP BY n.id, u.email
		ORDER BY n.created_at DESC
		LIMIT $2 OFFSET $3;
	`
	rows, err := r.db.Query(ctx, query, eventID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query admin history: %w", err)
	}
	defer rows.Close()

	var items []AdminNotificationHistoryItem
	for rows.Next() {
		var item AdminNotificationHistoryItem
		if err := rows.Scan(
			&item.ID, &item.Title, &item.Body, &item.ActionURL, &item.TargetType, &item.TargetID,
			&item.CreatedByEmail, &item.RecipientsCount, &item.ReadCount, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan history item: %w", err)
		}
		items = append(items, item)
	}

	if items == nil {
		items = []AdminNotificationHistoryItem{}
	}

	return &AdminHistoryResponse{
		Items:      items,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
	}, nil
}
