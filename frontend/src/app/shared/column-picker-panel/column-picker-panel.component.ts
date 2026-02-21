import {CdkDragDrop, DragDropModule, moveItemInArray} from '@angular/cdk/drag-drop';
import {Component, EventEmitter, Input, OnChanges, Output, SimpleChanges, signal} from '@angular/core';
import {FormsModule} from '@angular/forms';
import {ButtonModule} from 'primeng/button';

export interface ColumnPickerItem {
  id: string;
  label: string;
  example?: string;
}

@Component({
  selector: 'app-column-picker-panel',
  standalone: true,
  imports: [FormsModule, ButtonModule, DragDropModule],
  templateUrl: './column-picker-panel.component.html',
  styleUrl: './column-picker-panel.component.scss'
})
export class ColumnPickerPanelComponent implements OnChanges {
  @Input({required: true}) columns: readonly ColumnPickerItem[] = [];
  @Input({required: true}) selectedColumnIds: readonly string[] = [];
  @Input({required: true}) defaultColumnIds: readonly string[] = [];
  @Input() saving = false;

  @Output() cancel = new EventEmitter<void>();
  @Output() save = new EventEmitter<string[]>();
  @Output() dragStarted = new EventEmitter<void>();
  @Output() dragEnded = new EventEmitter<void>();

  editorColumns = signal<string[]>([]);
  search = signal('');

  private columnById = new Map<string, ColumnPickerItem>();

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['columns']) {
      this.columnById = new Map(this.columns.map(column => [column.id, column] as const));
    }

    if (changes['columns'] || changes['selectedColumnIds']) {
      this.editorColumns.set(this.resolveInitialColumns());
      this.search.set('');
    }
  }

  onSearchChange(value: string): void {
    this.search.set(value);
  }

  onDragStart(): void {
    this.dragStarted.emit();
  }

  onDragEnd(): void {
    this.dragEnded.emit();
  }

  resetColumns(): void {
    const normalizedDefaults = this.normalizeAgainstAvailable(this.defaultColumnIds);
    this.editorColumns.set(this.ensureVisibleColumn(normalizedDefaults));
  }

  onColumnDrop(event: CdkDragDrop<ColumnPickerItem[]>): void {
    if (this.columnSearchActive()) {
      return;
    }
    if (event.previousIndex === event.currentIndex) {
      return;
    }

    const columns = [...this.editorColumns()];
    moveItemInArray(columns, event.previousIndex, event.currentIndex);
    this.editorColumns.set(columns);
  }

  hideColumn(id: string): void {
    const current = this.editorColumns();
    if (current.length <= 1) {
      return;
    }
    this.editorColumns.set(current.filter(columnId => columnId !== id));
  }

  showColumn(id: string): void {
    const current = this.editorColumns();
    if (current.includes(id)) {
      return;
    }
    this.editorColumns.set([...current, id]);
  }

  hideAllColumns(): void {
    const current = this.editorColumns();
    if (current.length <= 1) {
      return;
    }
    this.editorColumns.set([current[0]]);
  }

  showAllColumns(): void {
    this.editorColumns.set(this.columns.map(column => column.id));
  }

  saveChanges(): void {
    const normalized = this.ensureVisibleColumn(this.normalizeAgainstAvailable(this.editorColumns()));
    this.save.emit(normalized);
  }

  visibleColumnsFiltered(): ColumnPickerItem[] {
    const search = this.normalizedSearch();
    const visibleColumns = this.visibleColumns();
    if (search.length === 0) {
      return visibleColumns;
    }
    return visibleColumns.filter(column => this.matchesSearch(column, search));
  }

  hiddenColumnsFiltered(): ColumnPickerItem[] {
    const search = this.normalizedSearch();
    const hiddenColumns = this.hiddenColumns();
    if (search.length === 0) {
      return hiddenColumns;
    }
    return hiddenColumns.filter(column => this.matchesSearch(column, search));
  }

  columnSearchActive(): boolean {
    return this.normalizedSearch().length > 0;
  }

  visibleColumnCount(): number {
    return this.editorColumns().length;
  }

  hiddenColumnCount(): number {
    return this.columns.length - this.editorColumns().length;
  }

  private visibleColumns(): ColumnPickerItem[] {
    const visible: ColumnPickerItem[] = [];

    for (const columnId of this.editorColumns()) {
      const definition = this.columnById.get(columnId);
      if (definition) {
        visible.push(definition);
      }
    }

    return visible;
  }

  private hiddenColumns(): ColumnPickerItem[] {
    const selected = new Set(this.editorColumns());
    return this.columns.filter(column => !selected.has(column.id));
  }

  private normalizedSearch(): string {
    return this.search().trim().toLowerCase();
  }

  private matchesSearch(column: ColumnPickerItem, search: string): boolean {
    const normalizedLabel = column.label.toLowerCase();
    const normalizedExample = column.example?.toLowerCase() ?? '';
    return normalizedLabel.includes(search) || normalizedExample.includes(search);
  }

  private resolveInitialColumns(): string[] {
    const normalizedSelected = this.normalizeAgainstAvailable(this.selectedColumnIds);
    if (normalizedSelected.length > 0) {
      return normalizedSelected;
    }

    const normalizedDefaults = this.normalizeAgainstAvailable(this.defaultColumnIds);
    return this.ensureVisibleColumn(normalizedDefaults);
  }

  private normalizeAgainstAvailable(ids: readonly string[]): string[] {
    const availableIds = new Set(this.columns.map(column => column.id));
    const seen = new Set<string>();
    const normalized: string[] = [];

    for (const id of ids) {
      if (!availableIds.has(id) || seen.has(id)) {
        continue;
      }
      seen.add(id);
      normalized.push(id);
    }

    return normalized;
  }

  private ensureVisibleColumn(ids: string[]): string[] {
    if (ids.length > 0) {
      return ids;
    }

    const firstColumnId = this.columns[0]?.id;
    return firstColumnId ? [firstColumnId] : [];
  }
}
