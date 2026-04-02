<script lang="ts">
	import type { TypeNode } from "./Node"
	import NodeCardBase from "./components/node/NodeCardBase.svelte"
	import DC01NodeCard from "./components/node/types/DC01NodeCard.svelte"
	import DC02NodeCard from "./components/node/types/DC02NodeCard.svelte"

	export let node: TypeNode
	export let showChart = false

	// Render is dispatched by node type so new node UIs can be added as standalone
	// templates instead of growing one generic component with many conditionals.
</script>

{#if node.TypeCode === 0xdc01}
	<DC01NodeCard {node} {showChart} />
{:else if node.TypeCode === 0xdc02}
	<DC02NodeCard {node} {showChart} />
{:else}
	<NodeCardBase {node} {showChart} metrics={node.Metrics ?? []} stats={node.Stats ?? []} />
{/if}
