import { SessionDetailView } from '@/components/SessionDetailView';

export default async function SessionDetailPage({
  params,
}: {
  params: Promise<{ date: string }>;
}) {
  const { date } = await params;
  return <SessionDetailView date={date} />;
}
