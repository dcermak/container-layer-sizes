import { type Writable, writable } from "svelte/store";
import { PageState, type Task } from "./types";

export const pageState: Writable<PageState> = writable(PageState.New);
export const activeTask: Writable<Task | undefined> = writable(undefined);
