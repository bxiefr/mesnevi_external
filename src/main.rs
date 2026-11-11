use eframe::egui;
use raw_window_handle::HasWindowHandle;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use windows::Win32::Foundation::HWND;
use windows::Win32::UI::Input::KeyboardAndMouse::GetAsyncKeyState;
use windows::Win32::UI::WindowsAndMessaging::{
    GetWindowLongPtrW, SetWindowLongPtrW, GWL_EXSTYLE, WS_EX_TRANSPARENT,
};

mod offset;
mod memory;
use memory::MemoryManager;

struct CheatApp {
    menu_open: Arc<AtomicBool>,
    box_esp: bool,
    skeleton_esp: bool,
    head_circle: bool,
    team_check: bool,
    health_bar: bool,
    armor_bar: bool,
    player_name: bool,
    distance_display: bool,
    hitmarker: bool,
    bomb_timer: bool,
    selected_theme: usize,
    
    memory: Option<MemoryManager>,
    last_connect_try: Instant,
}

impl Default for CheatApp {
    fn default() -> Self {
        let memory = MemoryManager::new("cs2.exe");
        Self {
            menu_open: Arc::new(AtomicBool::new(true)),
            box_esp: true,
            skeleton_esp: true,
            head_circle: true,
            team_check: true,
            health_bar: true,
            armor_bar: true,
            player_name: true,
            distance_display: true,
            hitmarker: true,
            bomb_timer: true,
            selected_theme: 0,
            memory,
            last_connect_try: Instant::now(),
        }
    }
}

impl eframe::App for CheatApp {
    fn clear_color(&self, _visuals: &egui::Visuals) -> [f32; 4] {
        [0.0, 0.0, 0.0, 0.0] // Şeffaf arka plan
    }

    fn update(&mut self, ctx: &egui::Context, frame: &mut eframe::Frame) {
        unsafe {
            if GetAsyncKeyState(0x2D) & 1 != 0 {
                let current = self.menu_open.load(Ordering::Relaxed);
                self.menu_open.store(!current, Ordering::Relaxed);
            }
        }

        let is_open = self.menu_open.load(Ordering::Relaxed);
        let transparent = !is_open;

        // Akıllı Tıklama Geçişi (Click-Through)
        if cfg!(target_os = "windows") {
            let _ = frame.window_handle().map(|wh| {
                if let raw_window_handle::RawWindowHandle::Win32(win32_handle) = wh.as_raw() {
                    let hwnd = HWND(win32_handle.hwnd.get() as _);
                    unsafe {
                        let ex_style = GetWindowLongPtrW(hwnd, GWL_EXSTYLE);
                        let is_currently_transparent = (ex_style & WS_EX_TRANSPARENT.0 as isize) != 0;
                        if transparent != is_currently_transparent {
                            let mut new_style = ex_style;
                            if transparent {
                                new_style |= WS_EX_TRANSPARENT.0 as isize;
                            } else {
                                new_style &= !(WS_EX_TRANSPARENT.0 as isize);
                            }
                            SetWindowLongPtrW(hwnd, GWL_EXSTYLE, new_style);
                        }
                    }
                }
            });
        }

        if self.memory.is_none() && self.last_connect_try.elapsed() > Duration::from_secs(3) {
            self.memory = MemoryManager::new("cs2.exe");
            self.last_connect_try = Instant::now();
        }

        // --- ESP ÇİZİM MOTORU ---
        let painter = ctx.layer_painter(egui::LayerId::background());
        let screen_size = ctx.screen_rect();

        if let Some(ref mem) = self.memory {
            let view_matrix = mem.get_view_matrix();
            
            // Örnek Test Çizimi (Sol üst köşede test kutusu)
            if self.box_esp {
                let test_pos = [0.0, 0.0, 0.0];
                if let Some(screen_pos) = MemoryManager::world_to_screen(&view_matrix, &test_pos, screen_size.width(), screen_size.height()) {
                    painter.rect_stroke(
                        egui::Rect::from_center_size(egui::pos2(screen_pos[0], screen_pos[1]), egui::vec2(50.0, 100.0)),
                        0.0,
                        // DÜZELTME: egui::Color yerine Color32 ve 1.5_f32 kullanıldı
                        egui::Stroke::new(1.5_f32, egui::Color32::from_rgb(0, 255, 0)),
                    );
                }
            }
        }

        // --- MENÜ ARAYÜZÜ ---
        if is_open {
            egui::Window::new("=== MESNEVI EXTERNAL (Rust) ===")
                .default_pos([100.0, 100.0])
                .resizable(false)
                .show(ctx, |ui| {
                    ui.vertical_centered(|ui| {
                        ui.heading("=== MESNEVI EXTERNAL (Rust) ===");
                    });
                    ui.separator();

                    ui.horizontal(|ui| {
                        ui.label("CS2 Durumu:");
                        if let Some(ref mem) = self.memory {
                            ui.colored_label(
                                egui::Color32::GREEN,
                                format!("BAĞLANDI (PID: {}, Base: 0x{:X})", mem.pid, mem.client_base),
                            );
                        } else {
                            ui.colored_label(egui::Color32::RED, "ARANIYOR (Oyun bekleniyor...)");
                        }
                    });
                    ui.separator();

                    ui.collapsing("ESP Ayarları", |ui| {
                        ui.checkbox(&mut self.box_esp, "Box ESP [F1]");
                        ui.checkbox(&mut self.skeleton_esp, "Skeleton ESP [F2]");
                        ui.checkbox(&mut self.head_circle, "Head Circle [F3]");
                        ui.checkbox(&mut self.team_check, "Team Check [F4]");
                    });

                    ui.collapsing("Görsel & Ek Bilgiler", |ui| {
                        ui.checkbox(&mut self.health_bar, "Health Bar [F5]");
                        ui.checkbox(&mut self.armor_bar, "Armor Bar [F9]");
                        ui.checkbox(&mut self.player_name, "Player Name [F6]");
                        ui.checkbox(&mut self.distance_display, "Distance Display [F7]");
                        ui.checkbox(&mut self.hitmarker, "Hitmarker & Sound [F10]");
                        ui.checkbox(&mut self.bomb_timer, "Bomb Timer [F12]");
                    });

                    ui.separator();
                    ui.horizontal(|ui| {
                        ui.label("ESP Tema Rengi:");
                        let themes = ["MAVİ", "YEŞİL", "KIRMIZI", "SARI", "MOR", "TURKUAZ"];
                        egui::ComboBox::from_id_source("theme_combo")
                            .selected_text(themes[self.selected_theme])
                            .show_ui(ui, |ui| {
                                for (idx, theme) in themes.iter().enumerate() {
                                    ui.selectable_value(&mut self.selected_theme, idx, *theme);
                                }
                            });
                    });

                    ui.add_space(15.0);
                    ui.separator();
                    ui.horizontal(|ui| {
                        if ui.button("💾 Config Kaydet").clicked() {}
                        if ui.button("❌ Panik Çıkış").clicked() {
                            std::process::exit(0);
                        }
                    });

                    ui.add_space(5.0);
                    ui.weak("[INSERT] Menüyü Gizle/Göster");
                });
        }

        ctx.request_repaint();
    }
}

fn main() -> eframe::Result<()> {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_decorations(false)
            .with_transparent(true)
            .with_always_on_top() // DÜZELTME: İçindeki argüman kaldırıldı
            .with_maximized(true),
        ..Default::default()
    };

    eframe::run_native(
        "MesneviOverlay",
        options,
        Box::new(|_cc| Box::<CheatApp>::default()),
    )
}
