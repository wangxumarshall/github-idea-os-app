import type { Metadata } from "next";
import { MulticaLanding } from "@/features/landing/components/multica-landing";

export const metadata: Metadata = {
  title: {
    absolute: "Multica — AI-Native Task Management",
  },
  description:
    "Open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills.",
  openGraph: {
    title: "Multica — AI-Native Task Management",
    description:
      "Manage your human + agent workforce in one place.",
    url: "/",
  },
  alternates: {
    canonical: "/",
  },
};

export default function LandingPage() {
  return <MulticaLanding />;
}
