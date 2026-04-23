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

<div class="flex gap-1 text-base-content">
	<div class="flex-0 border-2 rounded-xl border-base-300 bg-base-200">
		<div class="w-40 mt-2 flex flex-row gap-1 justify-center">
			{#each safeLeds as led}
				{#if led}
					<div class="w-3 h-3 m-1 rounded-full bg-base-content"></div>
				{:else}
					<div class="w-3 h-3 m-1 rounded-full border-2 border-base-300"></div>
				{/if}
			{/each}
		</div>
		<div class="w-40 m-1 flex justify-center">
			<span class="text-xl font-bold text-center">{node.Name}</span>
		</div>
		<div class="w-40 m-1 flex justify-center text-sm text-base-content/70">
			<span>{node.TypeName}</span>
		</div>
		<div class="w-40 m-1 px-2 flex flex-col gap-1">
			{#each safeMetricGroups as group, index}
				{@const metric = groupCurrentMetric(group)}
				{@const isSelected = selectedGroupIndex === index}
				<button
					class="flex items-center justify-between rounded-md border border-base-300 bg-base-100 px-2 py-1 text-left text-base-content transition-colors"
					class:bg-base-200={isSelected}
					on:click={() => selectGroup(index)}
					style:border-color={isSelected && metric ? metric.Color : undefined}
					style:box-shadow={isSelected && metric ? `0 0 0 1px ${metric.Color} inset` : undefined}
				>
					{#if isSingleMetric}
						<span class="w-full text-center font-bold">{metric ? `${metric.CurrentValue.toFixed(metric.Digit)} ${metric.Unit}` : "--"}</span>
					{:else}
						<span style="color: {metric?.Color ?? "inherit"}">{group.name}</span>
						<span class="font-bold">{metric ? `${metric.CurrentValue.toFixed(metric.Digit)} ${metric.Unit}` : "--"}</span>
					{/if}
				</button>
			{/each}
		</div>

		{#if safeStats.length > 0}
			<div class="w-40 m-1 px-2 flex flex-col gap-1">
				{#each safeStats as stat}
					<div class="rounded-md border border-base-300 bg-base-100 px-2 py-1 text-sm flex justify-between">
						<span class="text-base-content/70">{stat.Name}</span>
						<span class="font-bold">{stat.Value.toFixed(stat.Digit)} {stat.Unit}</span>
					</div>
				{/each}
			</div>
		{/if}

		<div class="w-40 m-1 flex justify-center">
			<div class="flex flex-col text-xs text-base-content/60">
				<span>{macAddress}</span>
				<span>RSSI:{node.RSSI.toFixed(0)}dB</span>
				<span>BATT:{node.Battery.toFixed(0)}%</span>
			</div>
		</div>
	</div>
	{#if showChart}
		<div class="flex-1 border-2 rounded-xl border-base-300 bg-base-100">
			{#key chartKey}
				<UPlot {series} {data} />
			{/key}
		</div>
	{/if}
</div>
