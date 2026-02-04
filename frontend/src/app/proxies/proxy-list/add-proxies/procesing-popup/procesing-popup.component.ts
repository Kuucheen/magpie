import {Component, EventEmitter, Input, OnChanges, OnDestroy, OnInit, Output, SimpleChanges, AfterViewInit, ElementRef, Inject} from '@angular/core';
import {DOCUMENT} from '@angular/common';
import {animate, style, transition, trigger} from '@angular/animations';
import {StarBackgroundComponent} from '../../../../ui-elements/star-background/star-background.component';


@Component({
    selector: 'app-procesing-popup',
    imports: [
    StarBackgroundComponent
],
    templateUrl: './procesing-popup.component.html',
    styleUrl: './procesing-popup.component.scss',
    animations: [
        trigger('textAnimation', [
            transition(':increment, :decrement', [
                style({ opacity: 0, transform: 'translateY(-10px)' }),
                animate('500ms ease-out', style({ opacity: 1, transform: 'translateY(0)' })),
            ]),
        ]),
    ]
})
export class ProcesingPopupComponent implements OnInit, OnChanges, OnDestroy, AfterViewInit {
  @Input() status: 'processing' | 'success' | 'error' = 'processing';
  @Input() count: number = 0;
  @Input() item: string = "";
  @Input() showDetailsButton = false;
  @Input() hideOverlay = false;
  @Input() autoCloseOnSuccess = true;
  @Output() closed = new EventEmitter<void>();
  @Output() detailsRequested = new EventEmitter<void>();

  messages: string[] = [];
  currentMessageIndex = 0;
  textState = 0;
  private messageRotationIntervalId: ReturnType<typeof setInterval> | null = null;
  private autoCloseTimeoutId: ReturnType<typeof setTimeout> | null = null;

  get currentMessage(): string {
    if (this.status !== 'processing') return '';
    return this.messages[this.currentMessageIndex];
  }

  constructor(
    private elementRef: ElementRef<HTMLElement>,
    @Inject(DOCUMENT) private document: Document
  ) {}

  ngOnInit() {
    this.initializeMessages();
    if (this.status === 'processing') {
      this.startMessageRotation();
    }
  }

  ngAfterViewInit(): void {
    const host = this.elementRef?.nativeElement;
    if (host && this.document?.body && host.parentElement !== this.document.body) {
      this.document.body.appendChild(host);
    }
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['item'] && !changes['item'].firstChange) {
      this.initializeMessages();
    }

    if (changes['status']) {
      const currentStatus: 'processing' | 'success' | 'error' = changes['status'].currentValue;

      if (!changes['status'].firstChange) {
        if (currentStatus === 'processing') {
          this.clearAutoCloseTimeout();
          this.startMessageRotation();
        } else {
          this.clearMessageRotation();

          if (currentStatus === 'success' && this.autoCloseOnSuccess) {
            this.scheduleAutoClose();
          } else {
            this.clearAutoCloseTimeout();
          }
        }
      } else if (currentStatus === 'success' && this.autoCloseOnSuccess) {
        this.scheduleAutoClose();
      }
    }

    if (changes['autoCloseOnSuccess'] && this.status === 'success') {
      if (this.autoCloseOnSuccess) {
        this.scheduleAutoClose();
      } else {
        this.clearAutoCloseTimeout();
      }
    }
  }

  ngOnDestroy(): void {
    this.clearMessageRotation();
    this.clearAutoCloseTimeout();
    const host = this.elementRef?.nativeElement;
    if (host?.parentElement === this.document?.body) {
      this.document.body.removeChild(host);
    }
  }

  startMessageRotation() {
    this.clearMessageRotation();

    if (!this.messages.length) {
      this.initializeMessages();
    }

    this.currentMessageIndex = 0;
    this.textState = 0;

    this.messageRotationIntervalId = setInterval(() => {
      if (this.status !== 'processing') {
        this.clearMessageRotation();
        return;
      }

      if (this.messages.length === 0) {
        return;
      }

      this.currentMessageIndex = (this.currentMessageIndex + 1) % this.messages.length;
      this.textState++;
    }, 10000);
  }

  onClose() {
    this.clearAutoCloseTimeout();
    this.closed.emit();
  }

  onDetails() {
    this.detailsRequested.emit();
  }

  private initializeMessages(): void {
    this.messages = [
      `Please wait while we add your ${this.item}.`,
      'This can take a few seconds.',
      'Just a little longer, we’re on it.',
      'Hang tight, we’re working on it.',
      'Almost there... just a moment more.',
      `Seems like you added a lot of ${ this.item }, but don’t worry, we’ll handle it.`,
      'Loading... good things take time!',
    ];
    this.currentMessageIndex = 0;
  }

  private clearMessageRotation(): void {
    if (this.messageRotationIntervalId !== null) {
      clearInterval(this.messageRotationIntervalId);
      this.messageRotationIntervalId = null;
    }
  }

  private scheduleAutoClose(delayMs: number = 2000): void {
    this.clearAutoCloseTimeout();
    this.autoCloseTimeoutId = setTimeout(() => this.onClose(), delayMs);
  }

  private clearAutoCloseTimeout(): void {
    if (this.autoCloseTimeoutId !== null) {
      clearTimeout(this.autoCloseTimeoutId);
      this.autoCloseTimeoutId = null;
    }
  }
}
