import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Orbex — Run anything. Know everything.",
  description: "Lightweight job orchestration platform. Run Docker containers with scheduling, monitoring, and real-time observability.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className="bg-zinc-950 text-zinc-100 antialiased min-h-screen">
        {children}
      </body>
    </html>
  );
}
