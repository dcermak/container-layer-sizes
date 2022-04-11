<script lang="ts">
  import { Route } from "tinro";
  import Main from "./Main.svelte";
  import History from "./History.svelte"
</script>

<nav>
  <a href="/">Home</a>
  <a href="/history">History</a>
</nav>

<Route path="/"><Main /></Route>
<Route path="/history"><History /></Route>
