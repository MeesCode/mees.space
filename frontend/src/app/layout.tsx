import type { Metadata, Viewport } from "next";
import "./globals.css";
import { Providers } from "@/components/Providers";
import { ClientLayout } from "@/components/ClientLayout";

export const metadata: Metadata = {
  title: "Mees Brinkhuis - System Architect",
  description:
    "The online business card of an enthusiastic guy who likes to fiddle around with computers.",
  icons: { icon: "/favicon-192.png", apple: "/favicon-192.png" },
};

export const viewport: Viewport = {
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
        <Providers>
          <ClientLayout>{children}</ClientLayout>
        </Providers>
        <div
          id="__ssr_data_slot__"
          aria-hidden="true"
          style={{ display: "none" }}
          dangerouslySetInnerHTML={{ __html: "<!--SSR_DATA-->" }}
        />
      </body>
    </html>
  );
}
