package app

import (
	"reflect"
	"testing"
)

func TestGetUniqueJiraAccountIDsFromText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "result-1",
			text: "[~accountid:557058:9cca305d-28f2-41c6-af4d-393b33527316] Не мог бы ты помочь с задачей " +
				"[~accountid:557058:9cca305d-28f2-41c6-af4d-393b33527316] Лучше всего тебе это увидеть" +
				"[~accountid:557058:9cca305d-28f2-41c6-af4d-393b33527316] Отревьюй",
			want: []string{"557058:9cca305d-28f2-41c6-af4d-393b33527316"},
		},
		{
			name: "result-2",
			text: "Не могли бы вы ответить на сообщение выше [~accountid:557058:9cca305d-28f2-41c6-af4d-393b33527316] [~accountid:5cfd562c515e2c0c5cfb40cc] " +
				"[~accountid:557058:9cca305d-28f2-41c6-af4d-393b33527316] этот комментарий к тебе не относится",
			want: []string{"557058:9cca305d-28f2-41c6-af4d-393b33527316", "5cfd562c515e2c0c5cfb40cc"},
		},
		{
			name: "result-3",
			text: "[~accountid:5c625e3cff6a2e4d3d1460a1] Подскажи как исправить ситуацию.  [~accountid:5cfd562c515e2c0c5cfb40cc], заапрувь " +
				"[~accountid:557058:9cca305d-28f2-41c6-af4d-393b33527316]  fyi",
			want: []string{"5c625e3cff6a2e4d3d1460a1", "5cfd562c515e2c0c5cfb40cc", "557058:9cca305d-28f2-41c6-af4d-393b33527316"},
		},
		{
			name: "result-4",
			text: "[~accountid:5cfd562c515e2c0c5cfb40cc] [~accountid:5c625e3cff6a2e4d3d1460a1] " +
				"[~accountid:5c667d34f8bb515c2c454660]  test",
			want: []string{"5cfd562c515e2c0c5cfb40cc", "5c625e3cff6a2e4d3d1460a1", "5c667d34f8bb515c2c454660"},
		},
		{
			name: "result-4",
			text: "[~accountid:5cfd562c515e2c0c5cfb40cc] [~accountid:5c625e3cff6a2e4d3d1460a1] [~accountid:5cfd562c515e2c0c5cfb40cc]" +
				"[~accountid:5c667d34f8bb515c2c454660] [~accountid:5c625e3cff6a2e4d3d1460a1] [~accountid:5c667d34f8bb515c2c454660] test",
			want: []string{"5cfd562c515e2c0c5cfb40cc", "5c625e3cff6a2e4d3d1460a1", "5c667d34f8bb515c2c454660"},
		},
		{
			name: "result-6",
			text: "Тестовый комментарий",
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getUniqueJiraAccountIDsFromText(tt.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getUniqueJiraAccountIDsFromText() = %v, want %v", got, tt.want)
			}
		})
	}
}
