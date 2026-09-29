<script lang="ts">
	import UPlot from "../../UPlot.svelte"
	import { onDestroy } from "svelte"
	import type { TypeMetric, TypeNode, TypeStat } from "../../Node"

	export let node: TypeNode
	export let metrics: TypeMetric[] = []
	export let stats: TypeStat[] = []
	export let showChart = false

	type MetricGroup = {
		key: string
		name: string
		metrics: TypeMetric[]
	}

	let selectedGroupIndex = 0
	$: safeMetrics = metrics ?? []
	$: safeStats = stats ?? []
	$: safeLeds = node?.Leds ?? []
	$: safeAxisX = node?.AxisX ?? []
	$: safeMetricGroups = buildMetricGroups(safeMetrics)
	$: isSingleMetric = safeMetricGroups.length <= 1
	$: if (selectedGroupIndex >= safeMetricGroups.length) selectedGroupIndex = 0
	$: selectedGroup = safeMetricGroups[selectedGroupIndex] ?? safeMetricGroups[0]
	$: selectedMetrics = selectedGroup?.metrics ?? []
	$: chartKey = selectedGroup ? `${selectedGroup.key}-${selectedMetrics.map((metric) => metric.Key).join("-")}` : "empty"
	$: series = selectedMetrics.length > 0
		? [
				{ label: "Time", value: (_: null, UTC: number) => (UTC == null ? "" : formatLocalTime(UTC)) },
				...selectedMetrics.map((metric) => ({
					label: metricSeriesLabel(metric),
					stroke: metric.Color,
					value: (_: null, rawValue: number) =>
						rawValue == null ? "" : rawValue.toFixed(metric.Digit) + metric.Unit,
				})),
		  ]
		: [{}, {}]

	$: macAddress = (node?.Mac ?? []).map((b) => b.toString(16).padStart(2, "0")).join(":").toUpperCase()
	let data: number[][] = [[], []]
	let displayedEndIndex = 0
	let targetEndIndex = 0
	let playbackTimer: number | null = null

	$: if (selectedMetrics.length > 0) {
		targetEndIndex = safeAxisX.length
		for (const metric of selectedMetrics) {
			targetEndIndex = Math.min(targetEndIndex, metric?.AxisY?.length ?? 0)
		}
		if (targetEndIndex < displayedEndIndex) {
			displayedEndIndex = targetEndIndex
		}
		data = [safeAxisX.slice(0, displayedEndIndex), ...selectedMetrics.map((metric) => (metric.AxisY ?? []).slice(0, displayedEndIndex))]
		ensurePlayback()
	} else {
		data = [[], []]
		displayedEndIndex = 0
		targetEndIndex = 0
	}

	function ensurePlayback() {
		if (playbackTimer != null || selectedMetrics.length === 0) return
		if (displayedEndIndex >= targetEndIndex) return
		let lastFrameAt = performance.now()
		const tick = () => {
			if (selectedMetrics.length === 0) {
				playbackTimer = null
				return
			}
			const now = performance.now()
			const sourceX = node?.AxisX ?? []
			targetEndIndex = sourceX.length
			for (const metric of selectedMetrics) {
				targetEndIndex = Math.min(targetEndIndex, metric?.AxisY?.length ?? 0)
			}
			if (displayedEndIndex >= targetEndIndex) {
				playbackTimer = null
				return
			}
			let steps = 1
			const prevIdx = Math.max(0, displayedEndIndex - 1)
			const nextIdx = Math.min(targetEndIndex - 1, displayedEndIndex)
			const deltaTs = Math.max(1, sourceX[nextIdx] - sourceX[prevIdx])
			const elapsed = Math.max(1, now - lastFrameAt)
			steps = Math.max(1, Math.floor(elapsed / deltaTs))
			displayedEndIndex = Math.min(targetEndIndex, displayedEndIndex + steps)
			data = [sourceX.slice(0, displayedEndIndex), ...selectedMetrics.map((metric) => (metric?.AxisY ?? []).slice(0, displayedEndIndex))]
			lastFrameAt = now
			playbackTimer = requestAnimationFrame(tick)
		}
		playbackTimer = requestAnimationFrame(tick)
	}

	onDestroy(() => {
		if (playbackTimer != null) cancelAnimationFrame(playbackTimer)
	})

	function formatLocalTime(timestamp: number | Date): string {
		const date = new Date(timestamp)
		const hours = date.getHours().toString().padStart(2, "0")
		const minutes = date.getMinutes().toString().padStart(2, "0")
		const seconds = date.getSeconds().toString().padStart(2, "0")
		return `${hours}:${minutes}:${seconds}`
	}

	function selectGroup(index: number) {
		selectedGroupIndex = index
	}

	function metricSeriesLabel(metric: TypeMetric) {
		const variant = metricVariantFromKey(metric.Key)
		switch (variant) {
			case "min":
				return "Min"
			case "avg":
				return "Avg"
			case "max":
				return "Max"
			default:
				return metric.Name
		}
	}

	function metricVariantFromKey(key: string): "min" | "avg" | "max" | "" {
		if (key.endsWith("Min")) return "min"
		if (key.endsWith("Avg")) return "avg"
		if (key.endsWith("Max")) return "max"
		return ""
	}

	function metricBaseKeyFromKey(key: string) {
		if (key.endsWith("Min") || key.endsWith("Avg") || key.endsWith("Max")) {
			return key.slice(0, -3)
		}
		return key
	}

	function metricBaseNameFromName(name: string) {
		return name.replace(/\s+(Min|Avg|Max)$/i, "")
	}

	function metricVariantOrder(metric: TypeMetric) {
		switch (metricVariantFromKey(metric.Key)) {
			case "min":
				return 0
			case "avg":
				return 1
			case "max":
				return 2
			default:
				return 3
		}
	}

	function buildMetricGroups(inputMetrics: TypeMetric[]): MetricGroup[] {
		const groups: MetricGroup[] = []
		const groupIndex = new Map<string, number>()
		for (const metric of inputMetrics) {
			const baseKey = metricBaseKeyFromKey(metric.Key)
			const baseName = metricBaseNameFromName(metric.Name)
			const existing = groupIndex.get(baseKey)
			if (existing == null) {
				groupIndex.set(baseKey, groups.length)
				groups.push({ key: baseKey, name: baseName, metrics: [metric] })
				continue
			}
			groups[existing].metrics = [...groups[existing].metrics, metric]
		}
		for (const group of groups) {
			group.metrics = [...group.metrics].sort((a, b) => metricVariantOrder(a) - metricVariantOrder(b))
		}
		return groups
	}

	function groupCurrentMetric(group: MetricGroup): TypeMetric | undefined {
		return group.metrics.find((metric) => metricVariantFromKey(metric.Key) === "avg") ?? group.metrics[0]
	}
</script>

<article class={`flex min-w-0 w-full flex-col gap-4 text-base-content ${showChart ? "sm:flex-row" : ""}`}>
	<div class={`flex min-w-0 w-full flex-col gap-4 rounded-xl border border-base-300 bg-base-200 p-4 ${showChart ? "sm:w-60 sm:shrink-0" : ""}`}>
		<div class="min-w-0">
			{#if node.TypeCode !== 0xdc04}
				<div class="mb-3 flex gap-2" aria-label="节点 LED 状态">
					{#each safeLeds as led}
						<div class="h-2.5 w-2.5 rounded-full" class:bg-base-content={led} class:border={!led} class:border-base-300={!led}></div>
					{/each}
				</div>
			{/if}
			<h2 class="break-words text-lg font-bold leading-tight">{node.Name}</h2>
			<p class="mt-1 text-sm text-base-content/70">{node.TypeName}</p>
		</div>

		<div class="flex flex-col gap-2">
			{#each safeMetricGroups as group, index}
				{@const metric = groupCurrentMetric(group)}
				{@const isSelected = selectedGroupIndex === index}
				<button
					class="flex min-h-11 min-w-0 items-center justify-between gap-2 rounded-lg border border-base-300 bg-base-100 px-3 py-2 text-left text-sm text-base-content transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-info"
					class:bg-base-200={isSelected}
					aria-pressed={isSelected}
					on:click={() => selectGroup(index)}
					style:border-color={isSelected && metric ? metric.Color : undefined}
				>
					<span class="min-w-0 break-words" style:color={metric?.Color}>{group.name}</span>
					<span class="shrink-0 font-bold tabular-nums">{metric ? `${metric.CurrentValue.toFixed(metric.Digit)} ${metric.Unit}` : "--"}</span>
				</button>
			{/each}
		</div>

		{#if safeStats.length > 0}
			<div class="border-t border-base-300 pt-3">
				{#each safeStats as stat}
					<div class="flex justify-between gap-2 text-sm">
						<span class="text-base-content/70">{stat.Name}</span>
						<span class="font-bold tabular-nums">{stat.Value.toFixed(stat.Digit)} {stat.Unit}</span>
					</div>
				{/each}
			</div>
		{/if}

		{#if node.TypeCode !== 0xdc04}
			<div class="mt-auto border-t border-base-300 pt-3 text-xs text-base-content/70">
				<div class="break-all font-mono">{macAddress}</div>
				<div class="mt-1 flex flex-wrap gap-x-3 tabular-nums">
					<span>RSSI {node.RSSI.toFixed(0)} dB</span>
					<span>电池 {node.Battery.toFixed(0)}%</span>
				</div>
			</div>
		{/if}
	</div>
	{#if showChart}
		<div class="h-72 min-w-0 flex-1 overflow-hidden rounded-xl border border-base-300 bg-base-200 p-3">
			{#key chartKey}
				<UPlot {series} {data} />
			{/key}
		</div>
	{/if}
</article>
