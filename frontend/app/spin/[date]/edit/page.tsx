import { SpinSessionForm } from '@/components/SpinSessionForm';

export default async function EditSpinSessionPage({
  params,
}: {
  params: Promise<{ date: string }>;
}) {
  const { date } = await params;
  return <SpinSessionForm mode="edit" date={date} />;
}
