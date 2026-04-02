<script lang="ts">
	import NodeCardBase from "../NodeCardBase.svelte"
	import type { TypeNode } from "../../../Node"

	export let node: TypeNode
	export let showChart = false

	// Keep type-specific selection local so future CM01 custom UI can evolve
	// without adding more branches into the generic node card.
	$: metrics = node.Metrics ?? []
	$: stats = (node.Stats ?? []).filter((item) => item.Key === "energyWh")
</script>

<NodeCardBase {node} {showChart} {metrics} {stats} />
