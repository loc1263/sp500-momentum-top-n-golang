package dateutil

import "testing"

func TestAddMonthsClampsEndOfMonth(t *testing.T) {
	got := Day(AddMonths(MustParse("2024-01-31"), 1))
	if got != "2024-02-29" {
		t.Fatalf("got %s", got)
	}
	if got := Day(AddYears(MustParse("2024-02-29"), -1)); got != "2023-02-28" {
		t.Fatalf("got %s", got)
	}
	if got := Day(AddMonths(MustParse("2024-03-15"), -4)); got != "2023-11-15" {
		t.Fatalf("got %s", got)
	}
}

func TestMonthStarts(t *testing.T) {
	got := MonthStarts(MustParse("2020-01-15"), MustParse("2020-12-31"), 4)
	want := []string{"2020-02-01", "2020-06-01", "2020-10-01"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if Day(got[i]) != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	// arranque justo en un inicio de mes: se incluye
	got = MonthStarts(MustParse("2020-01-01"), MustParse("2020-05-01"), 4)
	if len(got) != 2 || Day(got[1]) != "2020-05-01" {
		t.Fatalf("got %v", got)
	}
	if len(MonthStarts(MustParse("2020-01-15"), MustParse("2020-01-20"), 4)) != 0 {
		t.Fatal("se esperaba vacío")
	}
}
