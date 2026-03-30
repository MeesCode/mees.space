import type { Metadata } from "next";
import "./globals.css";
import { Sidebar } from "@/components/Sidebar";
import { Minimap } from "@/components/Minimap";

export const metadata: Metadata = {
  title: "Mees Brinkhuis - System Architect",
  description:
    "The online business card of an enthusiastic guy who likes to fiddle around with computers.",
  icons: { icon: "/favicon-192.png", apple: "/favicon-192.png" },
  themeColor: "#33ACB7",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en-us">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link
          href="https://fonts.googleapis.com/css2?family=Fira+Code&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <Sidebar />
        <main id="article-wrapper" className="app-container">
          {children}
        </main>
        <Minimap />
      </body>
    </html>
  );
}
