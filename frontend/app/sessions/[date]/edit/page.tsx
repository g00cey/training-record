import { StrengthSessionForm } from '@/components/StrengthSessionForm';

export default async function EditStrengthSessionPage({
  params,
}: {
  params: Promise<{ date: string }>;
}) {
  const { date } = await params;
  return <StrengthSessionForm mode="edit" date={date} />;
}
