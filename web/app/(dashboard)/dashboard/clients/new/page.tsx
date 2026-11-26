import type { Metadata } from "next";
import { NewClientForm } from "./new-client-form";

export const instant = false;
export const metadata: Metadata = { title: "New client | Photobooth", robots: { index: false, follow: false } };

export default function NewClientPage() {
  return <NewClientForm />;
}
