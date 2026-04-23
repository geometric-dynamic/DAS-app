<script lang="ts">
	import type { TypeDiscoveredNode } from "../../Node"

	export let discovered: TypeDiscoveredNode
	export let connecting = false
export let errorMessage = ""
	export let onConnect: () => void = () => {}
</script>

<div class="flex gap-1 text-base-content">
	<div class="flex-0 border-2 rounded-xl border-base-300 bg-base-200">
		<div class="w-40 mt-2 flex flex-row gap-1 justify-center">
			{#each Array(4) as _}
				<div class="w-3 h-3 m-1 rounded-full border-2 border-base-300"></div>
			{/each}
		</div>
		<div class="w-40 m-1 flex justify-center px-2">
			<span class="text-xl font-bold text-center break-all">{discovered.name}</span>
		</div>
		<div class="w-40 m-1 flex justify-center text-sm text-base-content/70 px-2 text-center">
			<span>{discovered.typeName}</span>
		</div>
		<div class="w-40 m-1 px-2 flex flex-col gap-1 text-sm text-base-content/70">
			<div class="rounded-md border border-base-300 bg-base-100 px-2 py-1 break-all">{discovered.ip}</div>
			<div class="rounded-md border border-base-300 bg-base-100 px-2 py-1 break-all">{discovered.nodeKey}</div>
		</div>
		<div class="w-40 m-1 px-2 py-2">
			<button class="btn btn-primary btn-sm w-full" disabled={connecting || discovered.connected} on:click={onConnect}>
				{#if connecting}
					连接中
				{:else if discovered.connected}
					已连接
				{:else}
					连接
				{/if}
			</button>
			{#if errorMessage}
				<div class="mt-1 text-[11px] text-error break-all">{errorMessage}</div>
			{/if}
		</div>
		<div class="w-40 m-1 flex justify-center">
			<div class="flex flex-col text-xs text-base-content/60">
				<span>{(discovered.mac ?? []).map((b) => b.toString(16).padStart(2, "0")).join(":").toUpperCase()}</span>
			</div>
		</div>
	</div>
</div>
