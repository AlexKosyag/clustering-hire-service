package notifications

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/store"
)

// Service — read-model уведомлений (NotificationsService в notifications.service.ts).
// Отдельной таблицы нет: источник истины — события предметной области (Invitation, TestSession).
type Service struct {
	db *store.DB
}

// NewService — конструктор для интегратора.
func NewService(db *store.DB) *Service { return &Service{db: db} }

// List — уведомления пользователя (list в TS).
// BUG-REF: G7 — статус «прочитано» не хранится и не изменяется, read выводится по правилам.
func (s *Service) List(ctx context.Context, userID string, role model.UserRole, limit int) (*ListResult, error) {
	items := []NotificationItem{}

	if role == model.RoleCandidate {
		profileID, err := s.candidateProfileID(ctx, userID)
		if err != nil {
			return nil, err
		}

		// До 30 последних приглашений по statusUpdatedAt.
		rows, err := s.db.Pool.Query(ctx, `
			SELECT i.id, i.status::text, i.salary_from, i.salary_to, i.created_at, i.status_updated_at, e.company_name
			FROM invitations i
			JOIN employer_profiles e ON e.id = i.employer_id
			WHERE i.candidate_id = $1
			ORDER BY i.status_updated_at DESC
			LIMIT 30`, profileID)
		if err != nil {
			return nil, err
		}

		type invitationRow struct {
			id              string
			status          string
			salaryFrom      int
			salaryTo        int
			createdAt       model.Time
			statusUpdatedAt model.Time
			companyName     string
		}
		invitations := []invitationRow{}
		for rows.Next() {
			var r invitationRow
			if err := rows.Scan(&r.id, &r.status, &r.salaryFrom, &r.salaryTo,
				&r.createdAt, &r.statusUpdatedAt, &r.companyName); err != nil {
				rows.Close()
				return nil, err
			}
			invitations = append(invitations, r)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()

		for _, inv := range invitations {
			switch inv.status {
			case string(model.InvSent):
				items = append(items, NotificationItem{
					ID:    inv.id,
					Type:  "INVITATION_RECEIVED",
					Title: fmt.Sprintf("Новое приглашение от «%s»", inv.companyName),
					Body: fmt.Sprintf("Предложение: %s — %s ₽",
						toLocaleStringRu(inv.salaryFrom), toLocaleStringRu(inv.salaryTo)),
					CreatedAt: inv.createdAt,
					Read:      false,
				})
			case string(model.InvAccepted), string(model.InvRejected), string(model.InvWithdrawn):
				items = append(items, NotificationItem{
					ID:        inv.id,
					Type:      "INVITATION_STATUS",
					Title:     fmt.Sprintf("Приглашение: %s", inv.status),
					Body:      fmt.Sprintf("«%s» — статус обновлён", inv.companyName),
					CreatedAt: inv.statusUpdatedAt,
					Read:      true,
				})
			default:
				continue // VIEWED и прочие не-терминальные статусы пропускаются
			}
		}

		// Последняя завершённая сессия тестирования.
		var (
			sessionID   string
			score       *float64
			gradeBefore *string
			gradeAfter  *string
			recommend   *string
			completedAt *model.Time
		)
		err = s.db.Pool.QueryRow(ctx, `
			SELECT id, score, grade_before::text, grade_after::text, recommendation, completed_at
			FROM test_sessions
			WHERE candidate_id = $1 AND status::text = 'COMPLETED'
			ORDER BY completed_at DESC
			LIMIT 1`, profileID).
			Scan(&sessionID, &score, &gradeBefore, &gradeAfter, &recommend, &completedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil && completedAt != nil {
			// Грейд: ${gradeBefore ?? '—'} → ${gradeAfter}, рекомендация ${recommendation}
			before := "—"
			if gradeBefore != nil {
				before = *gradeBefore
			}
			items = append(items, NotificationItem{
				ID:    sessionID,
				Type:  "TEST_RESULT",
				Title: fmt.Sprintf("Тестирование завершено: %s баллов", jsNumber(score)),
				Body: fmt.Sprintf("Грейд: %s → %s, рекомендация %s",
					before, jsValue(gradeAfter), jsValue(recommend)),
				CreatedAt: *completedAt,
				Read:      true,
			})
		}
	} else {
		employerID, err := s.employerProfileID(ctx, userID)
		if err != nil {
			return nil, err
		}

		rows, err := s.db.Pool.Query(ctx, `
			SELECT i.id, i.status::text, i.status_updated_at, c.first_name, c.last_name
			FROM invitations i
			JOIN candidate_profiles c ON c.id = i.candidate_id
			WHERE i.employer_id = $1 AND i.status::text <> 'SENT'
			ORDER BY i.status_updated_at DESC
			LIMIT 30`, employerID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				id              string
				status          string
				statusUpdatedAt model.Time
				firstName       *string
				lastName        *string
			)
			if err := rows.Scan(&id, &status, &statusUpdatedAt, &firstName, &lastName); err != nil {
				return nil, err
			}

			// [firstName, lastName].filter(Boolean).join(' ') || 'Кандидат'
			parts := make([]string, 0, 2)
			for _, p := range []*string{firstName, lastName} {
				if p != nil && *p != "" {
					parts = append(parts, *p)
				}
			}
			name := strings.Join(parts, " ")
			if name == "" {
				name = "Кандидат"
			}

			body := "Статус приглашения обновлён"
			if status == string(model.InvAccepted) {
				body = "Кандидат принял приглашение — контакты открыты в профиле"
			}

			items = append(items, NotificationItem{
				ID:        id,
				Type:      "INVITATION_STATUS",
				Title:     fmt.Sprintf("%s: %s", name, status),
				Body:      body,
				CreatedAt: statusUpdatedAt,
				Read:      status != string(model.InvAccepted),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Сортировка по createdAt desc (стабильная, как Array.prototype.sort) и обрезка до limit.
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.Time.After(items[j].CreatedAt.Time)
	})
	total := len(items)
	// items.slice(0, limit) в JS (отрицательный limit отрезает хвост).
	end := limit
	if end < 0 {
		end = len(items) + end
	}
	if end < 0 {
		end = 0
	}
	if end > len(items) {
		end = len(items)
	}
	items = items[:end]

	return &ListResult{Items: items, Total: total}, nil
}

// candidateProfileID — профиль кандидата по user_id (404 при отсутствии).
func (s *Service) candidateProfileID(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db.Pool.QueryRow(ctx, `SELECT id FROM candidate_profiles WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", common.NotFound("Профиль не найден")
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// employerProfileID — профиль работодателя по user_id (404 при отсутствии).
func (s *Service) employerProfileID(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db.Pool.QueryRow(ctx, `SELECT id FROM employer_profiles WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", common.NotFound("Профиль не найден")
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// toLocaleStringRu — (n).toLocaleString('ru-RU') для целых: группировка разрядов U+00A0 (NBSP).
func toLocaleStringRu(n int) string {
	sign := ""
	v := int64(n)
	if v < 0 {
		sign = "-"
		v = -v
	}
	digits := strconv.FormatInt(v, 10)
	var b strings.Builder
	b.WriteString(sign)
	for i := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteRune('\u00a0')
		}
		b.WriteByte(digits[i])
	}
	return b.String()
}

// jsNumber — String(number) в JS для score (85 → "85", 85.5 → "85.5").
func jsNumber(v *float64) string {
	if v == nil {
		return "null" // шаблонная строка `${null}` в JS
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// jsValue — подстановка nullable-значения в шаблонную строку JS (null/undefined → "null").
func jsValue(v *string) string {
	if v == nil {
		return "null"
	}
	return *v
}
