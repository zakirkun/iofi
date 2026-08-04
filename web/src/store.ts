import { create } from "zustand";
import { persist } from "zustand/middleware";

/** Toasts are the one piece of state every view writes and none owns. Before
 *  this, each panel kept its own `error` string and a failure raised outside a
 *  panel — a drag that a WIP limit refused, a background upload — had nowhere
 *  to appear. */
export type Toast = {
  id: number;
  tone: "info" | "good" | "bad";
  text: string;
};

type ToastState = {
  toasts: Toast[];
  push: (tone: Toast["tone"], text: string) => void;
  dismiss: (id: number) => void;
};

let nextToastID = 1;

export const useToasts = create<ToastState>((set) => ({
  toasts: [],
  push: (tone, text) => {
    const id = nextToastID++;
    set((s) => ({ toasts: [...s.toasts, { id, tone, text }] }));
    // Errors stay until dismissed; a failure that vanishes on its own is one
    // the founder can miss entirely while looking at another panel.
    if (tone !== "bad") {
      setTimeout(() => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })), 4000);
    }
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}));

/** toast is callable outside React — mutation error handlers and the SSE
 *  reducer are not components. */
export const toast = (tone: Toast["tone"], text: string) => useToasts.getState().push(tone, text);

export const reportError = (e: unknown) =>
  toast("bad", e instanceof Error ? e.message : String(e));

// ---------------------------------------------------------------- board UI

type BoardFilters = {
  query: string;
  roleId: string;
  label: string;
  unassignedOnly: boolean;
};

const emptyFilters: BoardFilters = { query: "", roleId: "", label: "", unassignedOnly: false };

type BoardState = {
  boardId: string | null;
  openTaskID: string | null;
  filters: BoardFilters;
  setBoard: (id: string) => void;
  openTask: (id: string | null) => void;
  setFilter: <K extends keyof BoardFilters>(key: K, value: BoardFilters[K]) => void;
  clearFilters: () => void;
};

export const useBoardUI = create<BoardState>()(
  persist(
    (set) => ({
      boardId: null,
      openTaskID: null,
      filters: emptyFilters,
      setBoard: (id) => set({ boardId: id, openTaskID: null }),
      openTask: (id) => set({ openTaskID: id }),
      setFilter: (key, value) => set((s) => ({ filters: { ...s.filters, [key]: value } })),
      clearFilters: () => set({ filters: emptyFilters }),
    }),
    {
      name: "iofi.board",
      // Only the board choice is worth surviving a reload. A persisted open
      // task would reopen a drawer for a task that may since have been deleted,
      // and persisted filters hide work with no visible reason why.
      partialize: (s) => ({ boardId: s.boardId }),
    },
  ),
);

export const filtersActive = (f: BoardFilters) =>
  !!f.query || !!f.roleId || !!f.label || f.unassignedOnly;

// ---------------------------------------------------------------- knowledge UI

type KnowledgeState = {
  openDocID: string | null;
  scope: "all" | "global" | "role" | "project";
  openDoc: (id: string | null) => void;
  setScope: (s: KnowledgeState["scope"]) => void;
};

export const useKnowledgeUI = create<KnowledgeState>((set) => ({
  openDocID: null,
  scope: "all",
  openDoc: (id) => set({ openDocID: id }),
  setScope: (scope) => set({ scope }),
}));
