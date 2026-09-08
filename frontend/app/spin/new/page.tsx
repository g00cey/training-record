import { SpinSessionForm } from '@/components/SpinSessionForm';

export default async function NewSpinSessionPage({
  searchParams,
}: {
  searchParams: Promise<{ date?: string }>;
}) {
  const { date } = await searchParams;
  return <SpinSessionForm mode="new" date={date} />;
}
