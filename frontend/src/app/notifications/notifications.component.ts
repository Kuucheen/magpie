import { CommonModule, DatePipe } from '@angular/common';
import { Component, OnInit, computed, signal } from '@angular/core';
import { Button } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { marked } from 'marked';
import { BuildInfo, ReleaseNote, UpdateNotificationService } from '../services/update-notification.service';
import { LoadingComponent } from '../ui-elements/loading/loading.component';

const RELEASE_SECTION_HEADINGS = new Set([
  "what's changed",
  'whats changed',
  'added',
  'improved',
  'fixed',
  'changed',
  'removed',
  'security',
  'breaking changes',
  'notes'
]);

function looksLikeMarkdown(body: string): boolean {
  return /(^|\n)\s{0,3}(#{1,6}\s|[-*+]\s|\d+\.\s|```|> )/.test(body) || /\[[^\]]+\]\([^)]+\)/.test(body);
}

function normalizeReleaseBodyToMarkdown(body: string): string {
  const normalized = body.replace(/\r\n/g, '\n').trim();
  if (!normalized || looksLikeMarkdown(normalized)) {
    return normalized;
  }

  const lines = normalized.split('\n').map((line) => line.trimRight());
  const output: string[] = [];
  let inListSection = false;

  for (const rawLine of lines) {
    const line = rawLine.trim();

    if (!line) {
      output.push('');
      inListSection = false;
      continue;
    }

    const lower = line.toLowerCase();
    if (RELEASE_SECTION_HEADINGS.has(lower)) {
      output.push(`${lower === "what's changed" || lower === 'whats changed' ? '##' : '###'} ${line}`);
      inListSection = lower !== "what's changed" && lower !== 'whats changed';
      continue;
    }

    if (inListSection) {
      output.push(`- ${line}`);
      continue;
    }

    output.push(line);
  }

  return output.join('\n');
}

@Component({
  selector: 'app-notifications',
  standalone: true,
  imports: [CommonModule, DatePipe, LoadingComponent, Button, DialogModule],
  templateUrl: './notifications.component.html',
  styleUrl: './notifications.component.scss'
})
export class NotificationsComponent implements OnInit {
  status = signal<{ loading: boolean; error?: string | null }>({ loading: true, error: null });
  newReleases = signal<ReleaseNote[]>([]);
  allReleases = signal<ReleaseNote[]>([]);
  lastSeenTag = signal<string | null>(null);
  latestTag = signal<string | null>(null);
  backendBuild = signal<BuildInfo | null>(null);
  selectedRelease = signal<ReleaseNote | null>(null);
  releaseDialogVisible = signal(false);
  readonly hasNewReleases = computed(() => this.newReleases().length > 0);
  readonly newReleaseCount = computed(() => this.newReleases().length);
  readonly totalReleaseCount = computed(() => this.allReleases().length);
  readonly buildVersionLabel = computed(() => this.backendBuild()?.buildVersion || 'dev');
  readonly archiveCountLabel = computed(() => {
    const count = this.totalReleaseCount();
    return `${count} release${count === 1 ? '' : 's'} in archive`;
  });
  readonly buildTimestamp = computed<Date | null>(() => {
    const builtAt = this.backendBuild()?.builtAt;
    if (!builtAt || builtAt === 'unknown') {
      return null;
    }

    const parsed = new Date(builtAt);
    return Number.isNaN(parsed.getTime()) ? null : parsed;
  });
  readonly selectedReleaseMarkdown = computed(() => {
    const release = this.selectedRelease();
    const body = normalizeReleaseBodyToMarkdown(release?.body?.trim() || 'No changelog text provided.');
    return marked.parse(body, {
      async: false,
      breaks: true,
      gfm: true
    }) as string;
  });

  constructor(private updates: UpdateNotificationService) {}

  ngOnInit(): void {
    this.loadReleases();
  }

  markAllSeen(): void {
    const latest = this.latestTag();
    if (!latest) {
      return;
    }

    this.updates.markAllSeen(latest);
    this.lastSeenTag.set(latest);
    this.newReleases.set([]);
  }

  retry(): void {
    this.loadReleases();
  }

  openReleaseDialog(release: ReleaseNote): void {
    this.selectedRelease.set(release);
    this.releaseDialogVisible.set(true);
  }

  onReleaseDialogVisibleChange(visible: boolean): void {
    this.releaseDialogVisible.set(visible);
    if (!visible) {
      this.selectedRelease.set(null);
    }
  }

  private loadReleases(): void {
    this.status.set({ loading: true, error: null });

    this.updates.fetchReleaseFeed().subscribe({
      next: (feed) => {
        this.newReleases.set(feed.newSinceLastSeen);
        this.allReleases.set(feed.releases);
        this.lastSeenTag.set(feed.lastSeenTag);
        this.latestTag.set(feed.latestTag);
        this.backendBuild.set(feed.backendBuild);
        this.status.set({ loading: false, error: null });
      },
      error: (err: Error) => {
        this.status.set({
          loading: false,
          error: err?.message ?? 'Failed to load release notes'
        });
      }
    });
  }
}
