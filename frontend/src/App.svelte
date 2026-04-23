<script lang="ts">
	import NodeRenderer from "./NodeRenderer.svelte"
	import DiscoveredNodeCard from "./components/node/DiscoveredNodeCard.svelte"
	import type { TypeAppState, TypeDiscoveredNode, TypeFrontendState, TypeNode } from "./Node"
	import { ConnectNode, GetFrontendState, MarkFrontendReady, StartGlobalStats, StartRecording, StartSim, StopGlobalStats, StopRecording, StopSim } from "../wailsjs/go/main/App"
	import { EventsOn } from '../wailsjs/runtime';
	import { onMount } from "svelte"

	let showChart = false
	let statsActive = false
	let recordingActive = false
	let appState: TypeAppState = { Simulating: false, HasExternalNodes: false }
	const headerStyle = "top-0 left-0 right-0 h-14"
	const footerStyle = "bottom-0 left-0 right-0 h-10"

	let nodes: TypeNode[] = []
	let discoveredNodes: TypeDiscoveredNode[] = []
	let connectPending: string[] = []
	let connectErrors: Record<string, string> = {}
	let showUdevDialog = false
	let udevFixCommand = ""
	const psurcUdevErrorTag = "PSURC_UDEV_PERMISSION"

	type DisplayCard =
		| { kind: "connected"; nodeKey: string; sortName: string; node: TypeNode }
		| { kind: "discovered"; nodeKey: string; sortName: string; discovered: TypeDiscoveredNode }

	$: connectedNodeKeys = new Set(nodes.map((node) => getNodeKey(node)))
	$: displayCards = buildDisplayCards(nodes, discoveredNodes)

	onMount(() => {
		StopSim()
		GetFrontendState().then((state) => {
			applyFrontendState(state)
		})
		EventsOn("new-data", (data) => {
			applyNodes(data)
		});
		EventsOn("discovered-nodes", (data) => {
			applyDiscoveredNodes(data)
		})
		EventsOn("app-state", (state) => {
			appState = normalizeAppState(state)
		})
		EventsOn("recording-state", (active) => {
			recordingActive = Boolean(active)
		})
		MarkFrontendReady()
	})

	function applyNodes(data: unknown) {
		nodes = Object.values((data ?? {}) as Record<string, TypeNode>) as TypeNode[]
	}

	function applyDiscoveredNodes(data: unknown) {
		discoveredNodes = (Object.values((data ?? {}) as Record<string, TypeDiscoveredNode>) as TypeDiscoveredNode[])
			.sort((a: TypeDiscoveredNode, b: TypeDiscoveredNode) => Number(b.online) - Number(a.online) || (a.name ?? "").localeCompare(b.name ?? ""))
	}

	function applyFrontendState(state: unknown) {
		const value = (state ?? {}) as TypeFrontendState
		applyNodes(value.Nodes)
		applyDiscoveredNodes(value.DiscoveredNodes)
		appState = normalizeAppState(value.AppState)
	}

	function getNodeKey(node: TypeNode): string {
		return (node.Mac ?? []).map((b) => b.toString(16).padStart(2, "0")).join("").toLowerCase()
	}

	function buildDisplayCards(nodes: TypeNode[], discoveredNodes: TypeDiscoveredNode[]): DisplayCard[] {
		const connectedCards: DisplayCard[] = nodes.map((node) => ({
			kind: "connected",
			nodeKey: getNodeKey(node),
			sortName: node.Name ?? "",
			node,
		}))
		const connectedNodeKeys = new Set(connectedCards.map((item) => item.nodeKey))
		const connectedMacKeys = new Set(nodes.map((node) => (node.Mac ?? []).map((b) => b.toString(16).padStart(2, "0")).join("").toLowerCase()))
		const discoveredCards: DisplayCard[] = discoveredNodes
			.filter((node) => {
				if (connectedNodeKeys.has(node.nodeKey)) return false
				const discoveredMac = (node.mac ?? []).map((b) => b.toString(16).padStart(2, "0")).join("").toLowerCase()
				if (discoveredMac && connectedMacKeys.has(discoveredMac)) return false
				return true
			})
			.map((discovered) => ({
				kind: "discovered",
				nodeKey: discovered.nodeKey,
				sortName: discovered.name ?? discovered.ip ?? discovered.nodeKey,
				discovered,
			}))

		return [...connectedCards, ...discoveredCards].sort((a, b) => {
			if (a.kind !== b.kind) return a.kind === "connected" ? -1 : 1
			return a.sortName.localeCompare(b.sortName)
		})
	}

	function normalizeAppState(state: unknown): TypeAppState {
		const value = (state ?? {}) as {
			Simulating?: boolean
			HasExternalNodes?: boolean
			simulating?: boolean
			hasExternalNodes?: boolean
		}
		return {
			Simulating: Boolean(value.Simulating ?? value.simulating),
			HasExternalNodes: Boolean(value.HasExternalNodes ?? value.hasExternalNodes),
		}
	}

	async function toggleSim() {
		if (appState.HasExternalNodes) return
		if (appState.Simulating) {
			await StopSim()
			appState = { ...appState, Simulating: false }
			return
		}
		await StartSim(16)
		appState = { ...appState, Simulating: true }
	}

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

	async function connectNode(nodeKey: string) {
		connectPending = [...connectPending, nodeKey]
		connectErrors = { ...connectErrors, [nodeKey]: "" }
		try {
			await ConnectNode(nodeKey)
		} catch (error) {
			console.error(error)
			const message = error instanceof Error ? error.message : String(error)
			connectErrors = { ...connectErrors, [nodeKey]: message || "连接失败" }
			maybeOpenUdevDialog(message || "")
		} finally {
			connectPending = connectPending.filter((key) => key !== nodeKey)
		}
	}

	function isConnecting(nodeKey: string) {
		return connectPending.includes(nodeKey)
	}

	function maybeOpenUdevDialog(message: string) {
		if (!message.includes(psurcUdevErrorTag)) return
		const parts = message.split("\n")
		if (parts.length >= 2) {
			udevFixCommand = parts.slice(1).join("\n").trim()
		} else {
			udevFixCommand = ""
		}
		showUdevDialog = true
	}

	async function copyUdevCommand() {
		if (!udevFixCommand) return
		try {
			await navigator.clipboard.writeText(udevFixCommand)
		} catch (error) {
			console.error(error)
		}
	}
</script>

<header
	class={`fixed ${headerStyle} bg-base-100/88 border-b border-base-300 text-base-content z-50 flex items-center px-4`}
>
	<div class="text-3xl font-bold select-none">DAS Console</div>

	<div class="ml-auto flex items-center gap-2">
		
		<button
			class="btn btn-outline btn-info btn-sm"
			on:click={toggleSim}
			title={appState.HasExternalNodes ? "真实节点在线时不可模拟" : appState.Simulating ? "Stop simulation" : "Start simulation"}
			disabled={appState.HasExternalNodes}
			class:btn-active={appState.Simulating}
		>
		<span class="">{appState.Simulating ? "停止模拟" : "开始模拟"}</span>
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
<main class="my-2 text-base-content">
	<div
		class="flex gap-1 flex-wrap"
		class:flex-col={showChart}
		class:flex-row={!showChart}
	>
		{#if displayCards.length === 0}
			<div class="mx-2 text-sm text-base-content/60">暂无发现到的 node</div>
		{:else}
			{#each displayCards as item (item.nodeKey)}
				{#if item.kind === "connected"}
					<NodeRenderer node={item.node} {showChart} />
				{:else}
					<DiscoveredNodeCard
						discovered={item.discovered}
						connecting={isConnecting(item.discovered.nodeKey)}
						errorMessage={connectErrors[item.discovered.nodeKey] ?? ""}
						onConnect={() => connectNode(item.discovered.nodeKey)}
					/>
				{/if}
			{/each}
		{/if}
	</div>
</main>
<div class={footerStyle}></div>
<footer
	class={`fixed ${footerStyle} border-t border-base-300 z-40 bg-base-100/88 text-base-content flex items-center px-4 gap-2`}
>
	<div class="text-sm text-base-content/70 select-none">节点数: {nodes.length}</div>
	<div class="ml-auto text-sm text-base-content/60 select-none">模拟: {appState.Simulating ? "进行中" : "未开始"} | 统计: {statsActive ? "进行中" : "未开始"} | 录制: {recordingActive ? "进行中" : "未开始"}</div>
</footer>

{#if showUdevDialog}
	<dialog class="modal modal-open">
		<div class="modal-box max-w-2xl">
			<h3 class="font-bold text-lg">USB-HID 权限不足</h3>
			<p class="py-2 text-sm text-base-content/80">请在终端执行以下命令后重新插拔设备，然后重启应用。</p>
			{#if udevFixCommand}
				<textarea class="textarea textarea-bordered w-full h-40 font-mono text-xs" readonly value={udevFixCommand}></textarea>
			{/if}
			<div class="modal-action">
				<button class="btn btn-outline btn-sm" on:click={copyUdevCommand}>复制命令</button>
				<button class="btn btn-primary btn-sm" on:click={() => (showUdevDialog = false)}>我已知晓</button>
			</div>
		</div>
	</dialog>
{/if}

<style>
	:global(html) {
		/* reserve the vertical scrollbar space so layout doesn't shift when content overflows */
		overflow-y: scroll;
	}
</style>
