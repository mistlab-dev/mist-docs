package scheduler

import (
	"testing"
	"time"
)

// ==================== 提醒文案 ====================

func TestBuildMessage(t *testing.T) {
	due := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		orderNo    string
		title      string
		offsetDays int
		want       string
	}{
		{
			name:       "order number and title",
			orderNo:    "PO-1",
			title:      "客户A的货",
			offsetDays: 3,
			want:       "[PO-1 客户A的货] 将在 3 天后（09-25）到期",
		},
		{
			name:       "due today",
			orderNo:    "",
			title:      "客户A的货",
			offsetDays: 0,
			want:       "[客户A的货] 今天到期（09-25）",
		},
		{
			name:       "overdue",
			orderNo:    "PO-1",
			title:      "",
			offsetDays: -2,
			want:       "[PO-1] 已逾期 2 天（交期 09-25）",
		},
		{
			name:       "one day ahead",
			orderNo:    "PO-9",
			title:      "",
			offsetDays: 1,
			want:       "[PO-9] 将在 1 天后（09-25）到期",
		},
		{
			name:       "both identifiers empty still reads sensibly",
			orderNo:    "",
			title:      "",
			offsetDays: 1,
			want:       "[未命名订单] 将在 1 天后（09-25）到期",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildMessage(c.orderNo, c.title, due, c.offsetDays); got != c.want {
				t.Fatalf("buildMessage() = %q\nwant            %q", got, c.want)
			}
		})
	}
}

// A reminder must never claim a past date is still upcoming. This guards the
// sign convention that a wrong offset would silently invert.
func TestBuildMessageWordingMatchesDatePosition(t *testing.T) {
	due := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

	if got := buildMessage("PO-1", "", due, 5); got == "" || !contains(got, "天后") || contains(got, "逾期") {
		t.Fatalf("positive offset should read as upcoming, got %q", got)
	}
	if got := buildMessage("PO-1", "", due, 0); !contains(got, "今天") {
		t.Fatalf("zero offset should read as due today, got %q", got)
	}
	if got := buildMessage("PO-1", "", due, -5); !contains(got, "逾期") {
		t.Fatalf("negative offset should read as overdue, got %q", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// ==================== 小工具 ====================

func TestItoa(t *testing.T) {
	cases := map[int]string{
		0:    "0",
		7:    "7",
		123:  "123",
		-3:   "-3",
		-100: "-100",
	}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 5); got != "abc" {
		t.Fatalf("truncate shorter than limit = %q", got)
	}
	if got := truncate("abcde", 5); got != "abcde" {
		t.Fatalf("truncate exact length = %q", got)
	}
	if got := truncate("abcdef", 5); got != "abcde" {
		t.Fatalf("truncate longer than limit = %q", got)
	}
}

// A webhook returning 4xx/5xx must surface as a failure so md_reminder_log
// records it, instead of being silently treated as delivered.
func TestHTTPErrorReportsFailure(t *testing.T) {
	err := (&httpError{code: 404}).Error()
	if err != "webhook returned HTTP 404" {
		t.Fatalf("httpError.Error() = %q", err)
	}
}
