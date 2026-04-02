<script lang="ts">
	import NodeRenderer from "./NodeRenderer.svelte"
	import type { TypeNode } from "./Node"
	import { StartGlobalStats, StartRecording, StartSim, StopGlobalStats, StopRecording, StopSim } from "../wailsjs/go/main/App.js"
	import { EventsOn } from '../wailsjs/runtime';
	import { onMount } from "svelte"

	let showChart = false
	let statsActive = false
	let recordingActive = false
	const headerStyle = "top-0 left-0 right-0 h-14"
	const footerStyle = "bottom-0 left-0 right-0 h-10"

	let nodes: TypeNode[] = []

	onMount(() => {
		StopSim()
		EventsOn("new-data", (data) => {
			nodes = []
			Object.values(data).forEach(node => {
				nodes.push(node as TypeNode)
			})
		});
		EventsOn("recording-state", (active) => {
			recordingActive = Boolean(active)
		})
	})

	async function toggleRecording() {
		if (recordingActive) {
			await StopRecording()
			recordingActive = false
			return
		}
		await StartRecording()
		recordingActive = true
	}

	async function toggleStats() {
		if (statsActive) {
			await StopGlobalStats()
			statsActive = false
			return
		}
		await StartGlobalStats()
		statsActive = true
	}
</script>

<header
	class={`fixed ${headerStyle} backdrop-blur-sm bg-black/30 border-b z-50 flex items-center px-4`}
>
	<div class="text-3xl font-bold">DAS Console</div>

	<div class="ml-auto flex items-center gap-2">
		
		<button
			class="btn btn-outline btn-info btn-sm"
			on:click={() => (StartSim(16))}
			title="Start simulation"
		>
		<span class="">Start Sim</span>
		</button>

		<button
			class="btn btn-outline btn-info btn-sm"
			on:click={StopSim}
			title="Stop simulation"
		>
		<span class="">Stop Sim</span>
		</button>

		<button class="btn btn-outline btn-warning btn-sm" class:btn-active={statsActive} on:click={toggleStats}>
			<span>{statsActive ? "结束统计" : "开始统计"}</span>
		</button>

		<button class="btn btn-outline btn-secondary btn-sm" class:btn-active={recordingActive} on:click={toggleRecording}>
			<span>{recordingActive ? "停止录制" : "开始录制"}</span>
		</button>

		<!-- Grid view button (active when showChart is false) -->
		<button
			class="btn btn-outline btn-info btn-sm"
			aria-pressed={!showChart}
			on:click={() => (showChart = false)}
			title="Grid view"
			class:btn-active={!showChart}
		>
			<!-- grid icon -->
			<svg
				xmlns="http://www.w3.org/2000/svg"
				class="w-5 h-5"
				viewBox="0 0 24 24"
				fill="currentColor"
				aria-hidden="true"
			>
				<path
					d="M3 3h8v8H3V3zm10 0h8v8h-8V3zM3 13h8v8H3v-8zm10 10V13h8v10h-8z"
				/>
			</svg>
			<span class="sr-only">Grid</span>
		</button>

		<!-- List / chart view button (active when showChart is true) -->
		<button
			class="btn btn-outline btn-info btn-sm"
			aria-pressed={showChart}
			on:click={() => (showChart = true)}
			title="List / Chart view"
			class:btn-active={showChart}
		>
			<!-- list icon -->
			<svg
				xmlns="http://www.w3.org/2000/svg"
				class="w-5 h-5"
				viewBox="0 0 24 24"
				fill="currentColor"
				aria-hidden="true"
			>
				<path d="M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h16v2H4v-2z" />
			</svg>
			<span class="sr-only">List</span>
		</button>
	</div>
</header>
<div class={headerStyle}></div>
<main class="my-2">
	<div
		class="flex gap-1 flex-wrap"
		class:flex-col={showChart}
		class:flex-row={!showChart}
	>
		{#each nodes as node}
			<NodeRenderer {node} {showChart} />
		{/each}
	</div>
</main>
<div class={footerStyle}></div>
<footer
	class={`fixed ${footerStyle} border-t z-40 backdrop-blur-sm bg-black/30 flex items-center px-4 gap-2`}
>
	<div class="text-sm text-gray-400">节点数: {nodes.length}</div>
	<div class="ml-auto text-sm text-gray-500">统计: {statsActive ? "进行中" : "未开始"} | 录制: {recordingActive ? "进行中" : "未开始"}</div>
</footer>

<style>
	:global(html) {
		/* reserve the vertical scrollbar space so layout doesn't shift when content overflows */
		overflow-y: scroll;
	}
</style>
