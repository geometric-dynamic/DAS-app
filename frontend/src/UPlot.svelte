<script lang="ts">
	import { onMount, onDestroy } from "svelte"
	import uPlot from "uplot"
	import "uplot/dist/uPlot.min.css"

	export let series = [{}, {}]
	export let data: number[][] = [[], []] // [x[], y[]]

	let container: HTMLDivElement // parent container <div>
	let chart: uPlot
	let resizeObserver: ResizeObserver
	let persistedYRange: [number, number] | null = null

	function createChart() {
		if (!container) return
		const rect = container.getBoundingClientRect()
		chart = new uPlot(
			{
				width: rect.width,
				height: rect.height,
				series,
				scales: {
					x: { time: true },
					y: {
						range: (_u, dataMin: number, dataMax: number) => getYRange(dataMin, dataMax),
					},
				},
				axes: [{ show: false }, { show: false }],
			},
			data.map((row) => new Float64Array(row)),
			container
		)
	}

	function getYRange(dataMin: number, dataMax: number): [number, number] {
		const yRows = data.slice(1)
		const yValues = yRows[0] ?? []
		const currentValue = yValues.length > 0 ? yValues[yValues.length - 1] : 0
		let safeMin = Number.isFinite(dataMin) ? dataMin : currentValue
		let safeMax = Number.isFinite(dataMax) ? dataMax : currentValue
		for (const row of yRows) {
			if (!row || row.length === 0) continue
			const rowLast = row[row.length - 1]
			if (Number.isFinite(rowLast)) {
				safeMin = Math.min(safeMin, rowLast)
				safeMax = Math.max(safeMax, rowLast)
			}
		}
		const minHalfRange = Math.max(1, Math.abs(currentValue) * 0.1)
		const targetLower = Math.min(safeMin, currentValue - minHalfRange)
		const targetUpper = Math.max(safeMax, currentValue + minHalfRange)

		if (targetLower === targetUpper) {
			const nextRange: [number, number] = [targetLower - minHalfRange, targetUpper + minHalfRange]
			persistedYRange = nextRange
			return nextRange
		}

		const previousRange = persistedYRange
		if (!previousRange) {
			const nextRange: [number, number] = [targetLower, targetUpper]
			persistedYRange = nextRange
			return nextRange
		}

		const previousSpan = Math.max(previousRange[1] - previousRange[0], minHalfRange * 2)
		const padding = Math.max(minHalfRange * 0.25, previousSpan * 0.05)
		const deadbandPadding = previousSpan * 0.2
		const shrinkFactor = 0.12
		const deadbandLower = previousRange[0] + deadbandPadding
		const deadbandUpper = previousRange[1] - deadbandPadding

		if (safeMin >= deadbandLower && safeMax <= deadbandUpper) {
			return previousRange
		}

		let nextLower = previousRange[0]
		let nextUpper = previousRange[1]

		if (targetLower < previousRange[0] + padding) {
			nextLower = targetLower - padding
		} else {
			nextLower = previousRange[0] + (targetLower - previousRange[0]) * shrinkFactor
		}

		if (targetUpper > previousRange[1] - padding) {
			nextUpper = targetUpper + padding
		} else {
			nextUpper = previousRange[1] + (targetUpper - previousRange[1]) * shrinkFactor
		}

		if (nextUpper - nextLower < minHalfRange * 2) {
			const center = (nextUpper + nextLower) / 2
			nextLower = center - minHalfRange
			nextUpper = center + minHalfRange
		}

		const nextRange: [number, number] = [nextLower, nextUpper]
		persistedYRange = nextRange

		return nextRange
	}

	function updateSize() {
		if (!chart) return
		const rect = container.getBoundingClientRect()
		chart.setSize({ width: rect.width, height: rect.height })
	}

	onMount(() => {
		createChart()

		// watch for parent resize
		resizeObserver = new ResizeObserver(updateSize)
		resizeObserver.observe(container)

		return () => {
			resizeObserver.disconnect()
			chart.destroy()
		}
	})

	// update data when parent passes new arrays
	$: if (chart) {
		chart.setData(data.map((row) => new Float64Array(row)))
	}
</script>

<!-- parent must have some CSS size for this div -->
<div bind:this={container} class="uplot-wrapper p-2"></div>

<style>
	.uplot-wrapper {
		/* ensures it grows/shrinks with parent */
		width: 95%;
		height: 60%;
	}
</style>
