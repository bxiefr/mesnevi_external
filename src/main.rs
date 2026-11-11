use eframe::egui;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use windows::Win32::Foundation::HWND;
use windows::Win32::UI::Input::KeyboardAndMouse::GetAsyncKeyState;
use windows::Win32::UI::WindowsAndMessaging::{
    GetWindowLongPtrW, SetWindowLongPtrW, GWL_EXSTYLE, WS_EX_TRANSPARENT,
};

struct CheatApp {
    // Menü State'leri (İleride Go'daki Config ile eşleyeceğimiz yerler)
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
}

impl Default for CheatApp {
    fn default() -> Self {
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
        }
    }
}

impl eframe::App for CheatApp {
    fn clear_color(&self, _visuals: &egui::Visuals) -> [f32; 4] {
        // Overlay penceremizin arkaplanını tamamen şeffaf yapıyoruz
        [0.0, 0.0, 0.0, 0.0]
    }

    fn update(&self, ctx: &egui::Context, frame: &mut eframe::Frame) {
        // Her frame'de INSERT tuşunu kontrol et (0x2D = VK_INSERT)
        unsafe {
            if GetAsyncKeyState(0x2D) & 1 != 0 {
                let current = self.menu_open.load(Ordering::Relaxed);
                self.menu_open.store(!current, Ordering::Relaxed);

                // Menü açıkken fare tıklamalarını al, kapalıyken tıklamayı oyuna geçir (Click-Through)
                if let Ok(info) = frame.info().native_handle {
                    if let eframe::NativeHandle::HWND(hwnd_ptr) = info {
                        let hwnd = HWND(hwnd_ptr as _);
                        let ex_style = GetWindowLongPtrW(hwnd, GWL_EXSTYLE);
                        if !current {
                            // Menü açıldı -> Tıklamaları yakala
                            SetWindowLongPtrW(hwnd, GWL_EXSTYLE, ex_style & !(WS_EX_TRANSPARENT.0 as isize));
                        } else {
                            // Menü kapandı -> Tıklamaları oyuna geçir
                            SetWindowLongPtrW(hwnd, GWL_EXSTYLE, ex_style | (WS_EX_TRANSPARENT.0 as isize));
                        }
                    }
                }
            }
        }

        // ESP Çizim Alanı (Ekranın Tamamı Üzerine Çizim)
        let painter = ctx.layer_painter(egui::LayerId::background());

        // Test için minik bir indikasyon (ESP çizimlerinin yapılacağı yer)
        if self.bomb_timer {
            painter.circle_stroke(
                egui::pos2(150.0, 150.0),
                15.0,
                egui::Stroke::new(2.0, egui::Color32::from_rgb(255, 50, 50)),
            );
            painter.text(
                egui::pos2(150.0, 125.0),
                egui::Align2::CENTER_CENTER,
                "39.5S",
                egui::FontId::proportional(13.0),
                egui::Color32::YELLOW,
            );
        }

        // --- egui Modern Menü ---
        if self.menu_open.load(Ordering::Relaxed) {
            egui::Window::new("=== MESNEVI EXTERNAL (Rust + egui) ===")
                .default_size([320.0, 420.0])
                .resizable(false)
                .show(ctx, |ui| {
                    ui.vertical_centered(|ui| {
                        ui.heading("ESP & Visual Settings");
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
                            // Config kaydetme mantığı
                        }
                        if ui.button("❌ Panik Çıkış").clicked() {
                            std::process::exit(0);
                        }
                    });

                    ui.add_space(5.0);
                    ui.weak("[INSERT] Menüyü Gizle/Göster");
                });
        }

        // Sürekli arayüz güncellemesi (60+ FPS Overlay için)
        ctx.request_repaint();
    }
}

fn main() -> eframe::Result<()> {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_decorations(false)
            .with_transparent(true)
            .with_always_on_top()
            .with_maximized(true),
        ..Default::default()
    };

    eframe::run_native(
        "MesneviOverlay",
        options,
        Box::new(|_cc| Box::<CheatApp>::default()),
    )
}
