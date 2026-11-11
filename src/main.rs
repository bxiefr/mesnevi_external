use eframe::egui;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use windows::Win32::UI::Input::KeyboardAndMouse::GetAsyncKeyState;

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
    
    // Bellek yöneticisi ve bağlantı kontrolü
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
    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        // INSERT tuşu ile menüyü gizle/göster
        unsafe {
            if GetAsyncKeyState(0x2D) & 1 != 0 {
                let current = self.menu_open.load(Ordering::Relaxed);
                self.menu_open.store(!current, Ordering::Relaxed);
            }
        }

        // Oyun açık değilse her 3 saniyede bir bağlanmayı dene (Auto-reconnect / arayüz donmaz)
        if self.memory.is_none() && self.last_connect_try.elapsed() > Duration::from_secs(3) {
            self.memory = MemoryManager::new("cs2.exe");
            self.last_connect_try = Instant::now();
        }

        if self.menu_open.load(Ordering::Relaxed) {
            egui::CentralPanel::default().show(ctx, |ui| {
                ui.vertical_centered(|ui| {
                    ui.heading("=== MESNEVI EXTERNAL (Rust) ===");
                });
                ui.separator();

                // Canlı Oyun Bağlantı Durumu
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
                    if ui.button("💾 Config Kaydet").clicked() {
                        // Config kaydetme
                    }
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
            .with_decorations(true)
            .with_always_on_top()
            .with_inner_size([380.0, 520.0]),
        ..Default::default()
    };

    eframe::run_native(
        "MesneviOverlay",
        options,
        Box::new(|_cc| Box::<CheatApp>::default()),
    )
}
