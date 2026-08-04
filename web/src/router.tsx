import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";
import App from "./App";
import HomeView from "./views/Home";
import BoardView from "./views/Board";
import OrgView from "./views/Org";
import MeetingsView from "./views/Meetings";
import KnowledgeView from "./views/Knowledge";
import RunsView from "./views/Runs";

// Routes are declared in code rather than generated from files: there are six
// of them, and a codegen step in the build is a cost with no purchaser.
//
// Each createRoute call is written out rather than looped through a helper —
// a helper taking `path: string` widens the literal, and the whole point of
// this router is that a typo in a <Link to="..."> fails to compile.
const rootRoute = createRootRoute({ component: App });

const routeTree = rootRoute.addChildren([
  createRoute({ getParentRoute: () => rootRoute, path: "/", component: HomeView }),
  createRoute({ getParentRoute: () => rootRoute, path: "/board", component: BoardView }),
  createRoute({ getParentRoute: () => rootRoute, path: "/org", component: OrgView }),
  createRoute({ getParentRoute: () => rootRoute, path: "/meetings", component: MeetingsView }),
  createRoute({ getParentRoute: () => rootRoute, path: "/knowledge", component: KnowledgeView }),
  createRoute({ getParentRoute: () => rootRoute, path: "/runs", component: RunsView }),
]);

export const router = createRouter({
  routeTree,
  // The Go handler serves index.html for any path it does not recognise, so a
  // real URL survives a reload and a bookmark lands on the right tab.
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
