import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { DateTimeInput } from "./DateTimeInput";

describe("DateTimeInput", () => {
  it("renders display format YYYY-MM-DD HH:mm from ISO local value", () => {
    const html = renderToStaticMarkup(
      <DateTimeInput aria-label="Start time" value="2026-10-19T14:30" onChange={vi.fn()} />,
    );

    expect(html).toContain('value="2026-10-19 14:30"');
    expect(html).toContain('placeholder="YYYY-MM-DD HH:mm"');
    expect(html).toContain('aria-label="Start time"');
    expect(html).toContain('type="datetime-local"');
    expect(html).toContain('value="2026-10-19T14:30"');
    expect(html).toContain('aria-label="Open calendar picker"');
  });

  it("renders space-formatted value directly", () => {
    const html = renderToStaticMarkup(
      <DateTimeInput aria-label="Start time" value="2026-10-25 09:15" onChange={vi.fn()} />,
    );

    expect(html).toContain('value="2026-10-25 09:15"');
    expect(html).toContain('value="2026-10-25T09:15"');
  });

  it("preserves the separator while entering the time", () => {
    const html = renderToStaticMarkup(
      <DateTimeInput aria-label="Start time" value="2026-10-19 " onChange={vi.fn()} />,
    );

    expect(html).toContain('value="2026-10-19 "');
  });

  it("disables both the text input and picker button when disabled", () => {
    const html = renderToStaticMarkup(
      <DateTimeInput
        aria-label="Start time"
        disabled
        value="2026-10-19 14:30"
        onChange={vi.fn()}
      />,
    );

    expect(html).toContain('disabled=""');
  });
});
