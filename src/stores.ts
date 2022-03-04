import { Writable, writable } from "svelte/store";
import { PageState, Task } from "./types";

export const pageState: Writable<PageState> = writable(PageState.New);
export const activeTask: Writable<Task | undefined> = writable(undefined);
