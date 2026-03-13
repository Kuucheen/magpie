import { CommonModule, DatePipe } from '@angular/common';
import { Component, OnInit, computed, signal } from '@angular/core';
import { Button } from 'primeng/button';
import { BuildInfo, ReleaseNote, UpdateNotificationService } from '../services/update-notification.service';
import { LoadingComponent } from '../ui-elements/loading/loading.component';

@Component({
  selector: 'app-notifications',
  standalone: true,
  imports: [CommonModule, DatePipe, LoadingComponent, Button],
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
