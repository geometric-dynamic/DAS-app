<script lang="ts">
	import type { TypeDiscoveredNode } from "../../Node"

export let discovered: TypeDiscoveredNode
export let connecting = false
export let errorMessage = ""
export let onConnect: () => void = () => {}
</script>

<article class="flex min-w-0 w-full flex-col gap-4 rounded-xl border border-base-300 bg-base-200 p-4 text-base-content">
	<div class="min-w-0">
		{#if discovered.typeCode !== 0xdc04}
			<div class="mb-3 flex gap-2" aria-hidden="true">
				{#each Array(4) as _}
					<div class="h-2.5 w-2.5 rounded-full border border-base-300"></div>
				{/each}
			</div>
		{/if}
		<h2 class="break-words text-lg font-bold leading-tight">{discovered.name}</h2>
		<p class="mt-1 text-sm text-base-content/70">{discovered.typeName}</p>
	</div>
	<div class="min-w-0 text-sm text-base-content/70">
		<div class="break-all">{discovered.ip}</div>
		{#if discovered.typeCode !== 0xdc04}
			<div class="mt-1 break-all font-mono text-xs">{discovered.nodeKey}</div>
		{/if}
	</div>
	<div class="mt-auto">
		<button class="btn btn-primary min-h-11 w-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-info" disabled={connecting || discovered.connected} on:click={onConnect}>
			{#if connecting}连接中{:else if discovered.connected}已连接{:else}连接{/if}
		</button>
		{#if errorMessage}
			<div class="mt-2 break-words text-xs text-error" role="alert">{errorMessage}</div>
		{/if}
	</div>
	{#if discovered.typeCode !== 0xdc04}
		<div class="border-t border-base-300 pt-3 font-mono text-xs text-base-content/70 break-all">
			{(discovered.mac ?? []).map((b) => b.toString(16).padStart(2, "0")).join(":").toUpperCase()}
		</div>
	{/if}
</article>
