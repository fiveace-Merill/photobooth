import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getDeliveryApi } from "@/lib/api";
import { DeliveryPage } from "@/components/delivery/delivery-page";

type Props = PageProps<"/g/[token]">;

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { token } = await params;
  const delivery = await getDeliveryApi(token).getDelivery(token);
  if (!delivery) return { title: "Link not found" };
  return {
    title: `${delivery.title} | ${delivery.studioName}`,
    robots: { index: false, follow: false },
  };
}

export default async function ClientDeliveryRoute({ params }: Props) {
  const { token } = await params;
  const delivery = await getDeliveryApi(token).getDelivery(token);
  if (!delivery) notFound();
  return <DeliveryPage delivery={delivery} token={token} />;
}
