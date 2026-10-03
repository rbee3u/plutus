import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import { Dialog } from "../../src/shared/ui/Dialog";
import "../../src/shared/styles/tokens.css";

function Fixture() {
  const [firstOpen, setFirstOpen] = useState(false);
  const [secondOpen, setSecondOpen] = useState(false);
  const [mounted, setMounted] = useState(true);
  const [firstMounted, setFirstMounted] = useState(true);
  return (
    <>
      <header style={{ position: "sticky", top: 0, background: "var(--surface)" }}>
        <button
          onClick={() => {
            setMounted(true);
            setFirstMounted(true);
            setFirstOpen(true);
          }}
        >
          Open first dialog
        </button>
      </header>
      <main style={{ minHeight: 2400 }}>
        <p id="scroll-marker">Background position marker</p>
      </main>
      {mounted && (
        <>
          {firstMounted && (
            <Dialog open={firstOpen} title="First dialog" onClose={() => setFirstOpen(false)}>
              <button onClick={() => setSecondOpen(true)}>Open second dialog</button>
              {Array.from({ length: 40 }, (_, index) => (
                <p key={index}>Scrollable dialog row {index}</p>
              ))}
            </Dialog>
          )}
          <Dialog open={secondOpen} title="Second dialog" onClose={() => setSecondOpen(false)}>
            <button onClick={() => setFirstMounted(false)}>Unmount first dialog</button>
            <button onClick={() => setMounted(false)}>Unmount all dialogs</button>
          </Dialog>
        </>
      )}
    </>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Fixture />
  </StrictMode>,
);
