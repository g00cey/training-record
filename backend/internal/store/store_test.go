package store_test

import (
	"errors"
	"testing"

	"training-record/internal/domain"
	"training-record/internal/store"
	"training-record/internal/testsupport"
)

func fptr(f float64) *float64 { return &f }
func iptr(i int) *int         { return &i }

func TestStrengthCRUD(t *testing.T) {
	st := testsupport.NewStore(t)

	// create
	s, err := st.CreateStrength("2026-09-01", "自重＋フリーウェイト", []domain.Exercise{
		{Name: "ヒップストラスト", Weight: fptr(40), Reps: 40, Sets: 1},
		{Name: "懸垂", Weight: nil, Reps: 10, Sets: 1},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(s.Exercises) != 2 || s.Exercises[0].SortOrder != 0 || s.Exercises[1].SortOrder != 1 {
		t.Fatalf("unexpected exercises: %+v", s.Exercises)
	}
	if s.CreatedAt == "" || s.UpdatedAt == "" {
		t.Errorf("timestamps not set: %+v", s)
	}

	// duplicate -> ErrDateExists
	if _, err := st.CreateStrength("2026-09-01", "", nil); !errors.Is(err, store.ErrDateExists) {
		t.Fatalf("want ErrDateExists, got %v", err)
	}

	// get
	got, err := st.GetStrengthByDate("2026-09-01")
	if err != nil || got == nil {
		t.Fatalf("get: %v / %v", got, err)
	}

	// missing -> (nil, nil)
	if got, err := st.GetStrengthByDate("2000-01-01"); err != nil || got != nil {
		t.Fatalf("missing get: %v / %v", got, err)
	}

	// replace
	rep, created, err := st.ReplaceStrength("2026-09-01", "自重のみ", []domain.Exercise{
		{Name: "腕立て伏せ", Reps: 30, Sets: 1},
	})
	if err != nil || created {
		t.Fatalf("replace: created=%v err=%v", created, err)
	}
	if len(rep.Exercises) != 1 || rep.Notes != "自重のみ" {
		t.Fatalf("replace result: %+v", rep)
	}

	// replace as upsert (new date -> created)
	_, created, err = st.ReplaceStrength("2026-09-02", "x", nil)
	if err != nil || !created {
		t.Fatalf("upsert new: created=%v err=%v", created, err)
	}

	// patch notes
	pn, err := st.UpdateStrengthNotes("2026-09-01", "自重のみ; 肩に違和感")
	if err != nil || pn == nil || pn.Notes != "自重のみ; 肩に違和感" {
		t.Fatalf("patch notes: %+v / %v", pn, err)
	}
	if got, err := st.UpdateStrengthNotes("2000-01-01", "x"); err != nil || got != nil {
		t.Fatalf("patch notes missing: %v / %v", got, err)
	}

	// append (existing session -> continues sort_order, joins notes)
	ap, err := st.AppendStrengthExercises("2026-09-01", []domain.Exercise{
		{Name: "ディップス", Reps: 20, Sets: 1},
	}, "追加分", true)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if len(ap.Exercises) != 2 || ap.Exercises[1].Name != "ディップス" || ap.Exercises[1].SortOrder != 1 {
		t.Fatalf("append order: %+v", ap.Exercises)
	}
	if ap.Notes != "自重のみ; 肩に違和感; 追加分" {
		t.Fatalf("append notes: %q", ap.Notes)
	}

	// append to a non-existent date -> creates
	ap2, err := st.AppendStrengthExercises("2026-09-03", []domain.Exercise{{Name: "懸垂", Reps: 6, Sets: 1}}, "新規", true)
	if err != nil || ap2 == nil || len(ap2.Exercises) != 1 {
		t.Fatalf("append create: %+v / %v", ap2, err)
	}

	// update exercise
	exID := ap.Exercises[0].ID
	up, err := st.UpdateExercise("2026-09-01", exID, domain.Exercise{Name: "腕立て伏せ", Reps: 35, Sets: 2})
	if err != nil {
		t.Fatalf("update exercise: %v", err)
	}
	if up.Exercises[0].Reps != 35 || up.Exercises[0].Sets != 2 {
		t.Fatalf("update exercise result: %+v", up.Exercises[0])
	}
	if _, err := st.UpdateExercise("2026-09-01", 999999, domain.Exercise{Name: "x", Reps: 1}); !errors.Is(err, store.ErrExerciseNotFound) {
		t.Fatalf("want ErrExerciseNotFound, got %v", err)
	}
	if _, err := st.UpdateExercise("2000-01-01", exID, domain.Exercise{Name: "x", Reps: 1}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// reorder
	ids := []int64{ap.Exercises[1].ID, ap.Exercises[0].ID}
	ro, err := st.ReorderExercises("2026-09-01", ids)
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if ro.Exercises[0].ID != ids[0] || ro.Exercises[1].ID != ids[1] {
		t.Fatalf("reorder result order: %+v", ro.Exercises)
	}
	if _, err := st.ReorderExercises("2026-09-01", []int64{999999}); !errors.Is(err, store.ErrExerciseNotFound) {
		t.Fatalf("reorder bad id: %v", err)
	}

	// delete exercise
	found, err := st.DeleteExercise("2026-09-01", ids[0])
	if err != nil || !found {
		t.Fatalf("delete exercise: found=%v err=%v", found, err)
	}
	found, err = st.DeleteExercise("2026-09-01", 999999)
	if err != nil || found {
		t.Fatalf("delete missing exercise: found=%v err=%v", found, err)
	}

	// list + total
	list, total, err := st.ListStrength("", "", 30, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("list total=%d len=%d", total, len(list))
	}
	if list[0].Date < list[1].Date {
		t.Fatalf("list not date-desc: %v", []string{list[0].Date, list[1].Date})
	}

	// list range + pagination
	list, total, err = st.ListStrength("2026-09-02", "2026-09-03", 1, 1)
	if err != nil {
		t.Fatalf("list range: %v", err)
	}
	if total != 2 || len(list) != 1 {
		t.Fatalf("range total=%d len=%d", total, len(list))
	}

	// delete session (cascade)
	found, err = st.DeleteStrength("2026-09-01")
	if err != nil || !found {
		t.Fatalf("delete session: found=%v err=%v", found, err)
	}
	found, err = st.DeleteStrength("2026-09-01")
	if err != nil || found {
		t.Fatalf("re-delete: found=%v err=%v", found, err)
	}
	var exCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM exercises e JOIN strength_sessions s ON s.id=e.session_id WHERE s.date='2026-09-01'`).Scan(&exCount); err != nil {
		t.Fatal(err)
	}
	if exCount != 0 {
		t.Fatalf("cascade left %d exercises", exCount)
	}
}

func TestSpinCRUDAndRPE(t *testing.T) {
	st := testsupport.NewStore(t)

	// create with explicit rpe
	sp, err := st.CreateSpin(domain.SpinSession{
		Date: "2026-07-11", DurationMinutes: 40, AvgHeartRate: iptr(155), MaxHeartRate: iptr(172), RPE: iptr(9),
	})
	if err != nil || sp.RPE == nil || *sp.RPE != 9 {
		t.Fatalf("create spin: %+v / %v", sp, err)
	}

	if _, err := st.CreateSpin(domain.SpinSession{Date: "2026-07-11", DurationMinutes: 1}); !errors.Is(err, store.ErrDateExists) {
		t.Fatalf("want ErrDateExists, got %v", err)
	}

	// replace (upsert new)
	_, created, err := st.ReplaceSpin(domain.SpinSession{Date: "2026-07-12", DurationMinutes: 46, AvgHeartRate: iptr(130), MaxHeartRate: iptr(167)})
	if err != nil || !created {
		t.Fatalf("replace new: created=%v err=%v", created, err)
	}

	// update full (used by PATCH) - missing -> (nil,nil)
	if got, err := st.UpdateSpinFull(domain.SpinSession{Date: "2000-01-01", DurationMinutes: 1}); err != nil || got != nil {
		t.Fatalf("update missing spin: %v / %v", got, err)
	}

	list, total, err := st.ListSpin("", "", 30, 0)
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("list spin: total=%d len=%d err=%v", total, len(list), err)
	}

	last, err := st.LastSpin()
	if err != nil || last == nil || last.Date != "2026-07-12" {
		t.Fatalf("last spin: %+v / %v", last, err)
	}

	found, err := st.DeleteSpin("2026-07-11")
	if err != nil || !found {
		t.Fatalf("delete spin: %v / %v", found, err)
	}
}

func TestRoutineStore(t *testing.T) {
	st := testsupport.NewStore(t)

	if d, err := st.LatestRoutineDate(); err != nil || d != "" {
		t.Fatalf("empty routine date: %q / %v", d, err)
	}

	if err := st.ReplaceRoutine("2026-07-03", []domain.RoutineExercise{
		{Name: "ヒップストラスト", Weight: fptr(38), Reps: 40, Sets: 1},
		{Name: "懸垂", Reps: 10, Sets: 1},
	}); err != nil {
		t.Fatalf("replace routine: %v", err)
	}
	if err := st.ReplaceRoutine("2026-07-30", []domain.RoutineExercise{
		{Name: "ヒップストラスト", Weight: fptr(40), Reps: 40, Sets: 1},
	}); err != nil {
		t.Fatalf("replace routine 2: %v", err)
	}

	d, err := st.LatestRoutineDate()
	if err != nil || d != "2026-07-30" {
		t.Fatalf("latest routine date: %q / %v", d, err)
	}

	hist, err := st.RoutineHistory()
	if err != nil || len(hist) != 2 || hist[0].Date != "2026-07-30" || hist[0].ExerciseCount != 1 {
		t.Fatalf("routine history: %+v / %v", hist, err)
	}

	// update existing exercise in the latest snapshot (in place)
	action, rdate, err := st.UpdateRoutineExercise("ヒップストラスト", fptr(42), nil, nil)
	if err != nil || action != "updated" || rdate != "2026-07-30" {
		t.Fatalf("update-exercise updated: %q %q %v", action, rdate, err)
	}
	rows, _ := st.RoutineByDate("2026-07-30")
	if len(rows) != 1 || rows[0].Weight == nil || *rows[0].Weight != 42 {
		t.Fatalf("update not applied: %+v", rows)
	}

	// unknown exercise -> appended
	action, _, err = st.UpdateRoutineExercise("新種目", nil, iptr(20), nil)
	if err != nil || action != "added" {
		t.Fatalf("update-exercise added: %q %v", action, err)
	}
	rows, _ = st.RoutineByDate("2026-07-30")
	if len(rows) != 2 || rows[1].Name != "新種目" || rows[1].Reps != 20 || rows[1].SortOrder != 1 {
		t.Fatalf("append result: %+v", rows)
	}

	// no routine at all -> ErrNotFound
	empty := testsupport.NewStore(t)
	if _, _, err := empty.UpdateRoutineExercise("x", nil, iptr(1), nil); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestProfileAndExerciseNames(t *testing.T) {
	st := testsupport.NewStore(t)

	p, err := st.GetProfile()
	if err != nil || p.BodyweightKg != 86 || p.HeightCm != 170 || p.MaxHrEst != 180 {
		t.Fatalf("default profile: %+v / %v", p, err)
	}
	p, err = st.UpdateProfile(fptr(85), nil, iptr(185))
	if err != nil || p.BodyweightKg != 85 || p.HeightCm != 170 || p.MaxHrEst != 185 {
		t.Fatalf("update profile: %+v / %v", p, err)
	}

	if _, err := st.CreateStrength("2026-09-01", "", []domain.Exercise{{Name: "懸垂", Reps: 10, Sets: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceRoutine("2026-09-01", []domain.RoutineExercise{{Name: "ディップス", Reps: 30, Sets: 1}}); err != nil {
		t.Fatal(err)
	}
	names, err := st.ExerciseNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "ディップス" || names[1] != "懸垂" {
		t.Fatalf("exercise names: %v", names)
	}
}

func TestVolumeLoadDomain(t *testing.T) {
	// bodyweight substitution
	if got := domain.ExerciseVolumeLoad("懸垂", nil, 10, 1, 86); got != 860 {
		t.Errorf("懸垂 vl = %v, want 860", got)
	}
	// non-listed bodyweight name -> 0
	if got := domain.ExerciseVolumeLoad("腕立て伏せ", nil, 30, 1, 86); got != 0 {
		t.Errorf("腕立て伏せ vl = %v, want 0", got)
	}
	// weighted
	if got := domain.ExerciseVolumeLoad("ヒップストラスト", fptr(40), 40, 1, 86); got != 1600 {
		t.Errorf("hip thrust vl = %v, want 1600", got)
	}
}
