import { StrengthSessionForm } from '@/components/StrengthSessionForm';

export default async function NewStrengthSessionPage({
  searchParams,
}: {
  searchParams: Promise<{ date?: string }>;
}) {
  const { date } = await searchParams;
  return <StrengthSessionForm mode="new" date={date} />;
}
