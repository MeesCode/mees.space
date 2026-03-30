"use client";

interface Props {
  path?: string;
}

export function TerminalPrompt({ path }: Props) {
  const displayPath = path && path !== "home" ? `~/${path}` : "~";

  return (
    <div className="terminal-prompt">
      <span className="prompt-text">visitor@mees.space</span>:{displayPath}${" "}
      <span className="blink">█</span>
    </div>
  );
}
