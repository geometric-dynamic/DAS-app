<script lang="ts">
	import NodeCardBase from "../NodeCardBase.svelte"
	import type { TypeNode } from "../../../Node"

	export let node: TypeNode
	export let showChart = false

	$: metrics = node.Metrics ?? []
	$: stats = (node.Stats ?? []).filter((item) => item.Key === "energyWh")
</script>

<NodeCardBase {node} {showChart} {metrics} {stats} />
