import { z } from 'zod';
import { HR_ZONES, isValidMmSs } from './domain';

export const dateSchema = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/, 'YYYY-MM-DD 形式で入力してください');

// --- 筋トレセッション ------------------------------------------------------

export const exerciseRowSchema = z
  .object({
    name: z.string().trim().min(1, '種目名は必須です'),
    bodyweight: z.boolean(),
    weight: z
      .union([z.coerce.number().nonnegative('0 以上で入力してください'), z.literal(''), z.null()])
      .optional(),
    reps: z.coerce.number().int('整数で入力してください').positive('1 以上で入力してください'),
    sets: z.coerce.number().int('整数で入力してください').positive('1 以上で入力してください'),
    notes: z.string().optional(),
  })
  .superRefine((val, ctx) => {
    if (!val.bodyweight) {
      if (val.weight === '' || val.weight === null || val.weight === undefined) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: '重量を入力してください（自重なら自重トグルを ON に）',
          path: ['weight'],
        });
      }
    }
  });

export const strengthFormSchema = z.object({
  date: dateSchema,
  notes: z.string().optional(),
  submitMode: z.enum(['replace', 'append']),
  exercises: z.array(exerciseRowSchema).min(1, '種目を 1 つ以上追加してください'),
});

export type StrengthFormValues = z.input<typeof strengthFormSchema>;

// --- スピンセッション ----------------------------------------------------

const optionalPositiveInt = z
  .union([z.coerce.number().int().positive(), z.literal(''), z.null()])
  .optional();

const hrZoneShape = HR_ZONES.reduce(
  (acc, z_) => {
    acc[z_] = z
      .string()
      .optional()
      .refine((v) => isValidMmSs(v ?? ''), 'mm:ss 形式で入力してください');
    return acc;
  },
  {} as Record<(typeof HR_ZONES)[number], z.ZodTypeAny>,
);

export const spinFormSchema = z.object({
  date: dateSchema,
  durationMinutes: z.coerce
    .number()
    .int('整数で入力してください')
    .positive('1 以上で入力してください'),
  avgHeartRate: optionalPositiveInt,
  maxHeartRate: optionalPositiveInt,
  rpe: z
    .union([z.coerce.number().int().min(1).max(10), z.literal(''), z.null()])
    .optional(),
  distanceKm: z
    .union([z.coerce.number().nonnegative(), z.literal(''), z.null()])
    .optional(),
  freeNotes: z.string().optional(),
  hrZones: z.object(hrZoneShape),
});

export type SpinFormValues = z.input<typeof spinFormSchema>;

// --- ルーティン --------------------------------------------------------

export const routineRowSchema = z.object({
  name: z.string().trim().min(1, '種目名は必須です'),
  bodyweight: z.boolean(),
  weight: z
    .union([z.coerce.number().nonnegative(), z.literal(''), z.null()])
    .optional(),
  reps: z.coerce.number().int().positive('1 以上で入力してください'),
  sets: z.coerce.number().int().positive('1 以上で入力してください'),
});

export const routineFormSchema = z.object({
  date: z
    .union([dateSchema, z.literal('')])
    .optional(),
  exercises: z.array(routineRowSchema).min(1, '種目を 1 つ以上追加してください'),
});

export type RoutineFormValues = z.input<typeof routineFormSchema>;

// --- プロフィール ------------------------------------------------------

export const profileFormSchema = z.object({
  bodyweightKg: z
    .union([z.coerce.number().positive(), z.literal(''), z.null()])
    .optional(),
  heightCm: z
    .union([z.coerce.number().positive(), z.literal(''), z.null()])
    .optional(),
  maxHrEst: z
    .union([z.coerce.number().int().positive(), z.literal(''), z.null()])
    .optional(),
});

export type ProfileFormValues = z.input<typeof profileFormSchema>;

// --- 変換ヘルパ ------------------------------------------------------

/** フォームの数値フィールド（"" / null / number）を number | null に正規化。 */
export function numOrNull(v: unknown): number | null {
  if (v === '' || v === null || v === undefined) return null;
  const n = typeof v === 'number' ? v : Number(v);
  return Number.isFinite(n) ? n : null;
}
