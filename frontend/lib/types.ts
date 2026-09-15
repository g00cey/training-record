// backend REST API (docs/api.md) のレスポンス型。JSON キーは camelCase。

export type ErrorBody = {
  error: { code: string; message: string };
};

export type ExerciseEntry = {
  id: number;
  name: string;
  weight: number | null;
  reps: number;
  sets: number;
  notes: string;
  sortOrder: number;
};

export type StrengthSession = {
  date: string;
  notes: string;
  createdAt?: string;
  updatedAt?: string;
  exercises: ExerciseEntry[];
};

export type StrengthSessionList = {
  items: StrengthSession[];
  total: number;
};

export type SpinSession = {
  date: string;
  durationMinutes: number;
  avgHeartRate: number | null;
  maxHeartRate: number | null;
  rpe: number | null;
  distanceKm: number | null;
  notes: string;
  createdAt?: string;
  updatedAt?: string;
};

export type SpinSessionList = {
  items: SpinSession[];
  total: number;
};

// --- スピン画像抽出（POST /api/spin-extract・Phase 7） ---
// hrZones のキーは lib/domain.ts の HR_ZONES 5 ゾーン（常に全キー存在、
// 未抽出は null）。date / rpe は抽出対象外（フォーム入力・自動算出のため）。

export type SpinExtractResult = {
  durationMinutes: number | null;
  avgHeartRate: number | null;
  maxHeartRate: number | null;
  distanceKm: number | null;
  hrZones: Record<string, string | null>;
  freeNotes: string;
  uncertainFields: string[];
};

export type RoutineExercise = {
  id: number;
  name: string;
  weight: number | null;
  reps: number;
  sets: number;
  sortOrder: number;
};

// 旧・互換読み取りビュー（GET /api/routine）。書き込みは presets に一本化済み。
export type Routine = {
  date: string;
  presets?: string[];
  exercises: RoutineExercise[];
};

export type RoutineHistory = {
  snapshots: { date: string; exerciseCount: number }[];
};

// --- 名前付きプリセット（GET /api/presets ほか） ---

export type PresetSummary = {
  name: string;
  sortOrder: number;
  exerciseCount: number;
  latestDate: string | null;
};

export type PresetList = {
  presets: PresetSummary[];
};

export type Preset = {
  name: string;
  date: string;
  exercises: RoutineExercise[];
};

export type PresetHistory = {
  snapshots: { date: string; exerciseCount: number }[];
};

export type ExercisesList = {
  exercises: string[];
};

export type Profile = {
  bodyweightKg: number | null;
  heightCm: number | null;
  maxHrEst: number | null;
};

export type SessionKind =
  | 'bodyweight_only'
  | 'fw_only'
  | 'bodyweight_and_fw'
  | 'other';

export type CalendarDay = {
  date: string;
  strength: {
    kind: SessionKind;
    exerciseCount: number;
    volumeLoad: number;
  } | null;
  spin: {
    durationMinutes: number;
    rpe: number | null;
  } | null;
};

export type CalendarResponse = {
  month: string;
  days: CalendarDay[];
};

export type VolumeSessionPoint = {
  date: string;
  volumeLoad: number;
  exerciseCount: number;
};

export type VolumeWeekPoint = {
  weekStart: string;
  volumeLoad: number;
  sessionCount: number;
};

export type VolumeExercisePoint = {
  date: string;
  weight: number | null;
  reps: number;
  sets: number;
  volumeLoad: number;
};

export type VolumeSessionResponse = {
  granularity: 'session';
  points: VolumeSessionPoint[];
};

export type VolumeWeekResponse = {
  granularity: 'week';
  points: VolumeWeekPoint[];
};

export type VolumeExerciseResponse = {
  exercise: string;
  points: VolumeExercisePoint[];
};

export type LoadReportZone =
  | 'safe'
  | 'caution'
  | 'warning'
  | 'low'
  | 'slightly_low'
  | 'no_data';

export type LoadReport = {
  asOf: string;
  profile: { bodyweightKg: number | null; heightCm: number | null; bmi: number | null };
  acute7d: {
    strengthSessions: number;
    spinSessions: number;
    volumeLoad: number;
    spinTrimp: number;
    total: number;
    volumeLoadPerBw: number;
  };
  chronic28d: { strengthSessions: number; weeklyVolumeLoad: number };
  acwr: number;
  zone: LoadReportZone;
  latestSessionBreakdown: { name: string; volumeLoad: number; isBodyweight: boolean }[];
  recommendations: string[];
};

export type WeeklySummary = {
  period: string;
  summary: {
    totalTrainingSessions: number;
    spinSessions?: Record<string, unknown>;
    strengthSessions?: Record<string, unknown>;
  };
  evaluation: string[];
  advice: string[];
};

export type Summary = {
  periodDays: number;
  since: string;
  strengthSessionsInPeriod: number;
  spinSessionsInPeriod: number;
  totalStrengthSessions: number;
  totalSpinSessions: number;
  latestStrengthDate: string | null;
  latestSpinDate: string | null;
  latestRoutineDate: string | null;
};

export type LastSession = {
  strength: StrengthSession | null;
  spin: SpinSession | null;
};

// --- 故障予防アドバイス（GET /api/advice・Phase 6） ---

export type FrequencyStatus = 'good' | 'low' | 'rest_needed' | 'long_off';
export type OverloadStatus = 'ok' | 'caution' | 'warning';

export type AdviceOverloadExercise = {
  name: string;
  prevWeight: number | null;
  latestWeight: number | null;
  changePct: number | null;
  status: OverloadStatus;
  lastIncreasedOn?: string | null;
};

export type AdviceWatchExercise = {
  name: string;
  reason: string;
  from: number | null;
  to: number | null;
  on: string | null;
  formGuideAnchor?: string;
};

export type AdviceFlaggedSession = {
  date: string;
  matched: string[];
  notes: string;
};

export type Advice = {
  asOf: string;
  acwr: {
    value: number;
    zone: LoadReportZone;
    acute7d: number;
    chronicWeekly: number;
  };
  frequency: {
    windowDays: number;
    strengthSessions: number;
    spinSessions: number;
    total: number;
    maxConsecutiveWithin24h: number;
    status: FrequencyStatus;
    message: string;
  };
  progressiveOverload: {
    weeklyVolumeChangePct: number | null;
    status: OverloadStatus;
    exercises: AdviceOverloadExercise[];
  };
  deload: {
    lastDeloadDate: string | null;
    weeksSince: number | null;
    due: boolean;
    message: string;
  };
  watchExercises: AdviceWatchExercise[];
  warningSigns: {
    flaggedSessions: AdviceFlaggedSession[];
    stopNow: string[];
    monitor: string[];
  };
};
