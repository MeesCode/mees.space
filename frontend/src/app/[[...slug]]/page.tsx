import { ContentPage } from "./ContentPage";

export function generateStaticParams() {
  return [{ slug: [] }];
}

export default function Page() {
  return <ContentPage />;
}
