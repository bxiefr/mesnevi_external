use eframe::egui;
use raw_window_handle::HasWindowHandle;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use windows::Win32::Foundation::HWND;
use windows::Win32::UI::Input::KeyboardAndMouse::GetAsyncKeyState;
use windows::Win32::UI::WindowsAndMessaging::{
    GetWindowLongPtrW, SetWindowLongPtrW, GWL_EXSTYLE, WS_EX_LAYERED, WS_EX_TRANSPARENT,
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
        if let Some(ref mem) = memory {
            println!("[+] CS2 Başarıyla Bağlandı! PID: {}, Base: 0x{:X}", mem.pid, mem.client_base);
        } else {
            println!("[-] CS2 Bulunamadı, oyun bekleniyor...");
        }
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
        [0.0, 0.0, 0.0, 0.0]
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

        if cfg!(target_os = "windows") {
            let _ = frame.window_handle().map(|wh| {
                if let raw_window_handle::RawWindowHandle::Win32(win32_handle) = wh.as_raw() {
                    let hwnd = HWND(win32_handle.hwnd.get() as _);
                    unsafe {
                        let ex_style = GetWindowLongPtrW(hwnd, GWL_EXSTYLE);
                        let mut new_style = ex_style;
                        new_style |= WS_EX_LAYERED.0 as isize;

                        if transparent {
                            new_style |= WS_EX_TRANSPARENT.0 as isize;
                        } else {
                            new_style &= !(WS_EX_TRANSPARENT.0 as isize);
                        }

                        if new_style != ex_style {
                            SetWindowLongPtrW(hwnd, GWL_EXSTYLE, new_style);
                        }
                    }
                }
            });
        }

        if self.memory.is_none() && self.last_connect_try.elapsed() > Duration::from_secs(3) {
            self.memory = MemoryManager::new("cs2.exe");
            if let Some(ref mem) = self.memory {
                println!("[+] CS2 Yeniden Bağlandı! PID: {}, Base: 0x{:X}", mem.pid, mem.client_base);
            }
            self.last_connect_try = Instant::now();
        }

        // --- ESP ÇİZİM DÖNGÜSÜ ---
        let painter = ctx.layer_painter(egui::LayerId::background());
        let screen_size = ctx.screen_rect();

        if let Some(ref mem) = self.memory {
            let view_matrix = mem.get_view_matrix();
            let local_controller = mem.read_pointer(mem.client_base + offset::DW_LOCAL_PLAYER_CONTROLLER);
            
            if local_controller != 0 {
                let local_pawn: usize = mem.read_pointer(local_controller + offset::M_H_PLAYER_PAWN as usize); // veya ilgili pawn offseti
                let local_team: i32 = mem.read(local_controller + offset::M_I_TEAM_NUM);
                let entity_list = mem.read_pointer(mem.client_base + offset::DW_ENTITY_LIST);

                if entity_list != 0 {
                    let list_entry = mem.read_pointer(entity_list + 0x10);
                    if list_entry != 0 {
                        for i in 1..64 {
                            let controller_base = mem.read_pointer(list_entry + i * 0x78);
                            if controller_base == 0 || controller_base == local_controller { continue; }

                            let pawn_handle: u32 = mem.read(controller_base + offset::M_H_PLAYER_PAWN);
                            if pawn_handle == 0 { continue; }

                            let list_entry2 = mem.read_pointer(entity_list + 0x8 * ((pawn_handle as usize & 0x7FFF) >> 9) + 16);
                            if list_entry2 == 0 { continue; }

                            let player_pawn = mem.read_pointer(list_entry2 + 0x78 * (pawn_handle as usize & 0x1FF));
                            if player_pawn == 0 || player_pawn == local_pawn { continue; }

                            let player_health: i32 = mem.read(player_pawn + offset::M_I_HEALTH);
                            if player_health <= 0 || player_health > 100 { continue; }

                            let player_team: i32 = mem.read(player_pawn + offset::M_I_TEAM_NUM);
                            if self.team_check && player_team == local_team { continue; }

                            // DEBUG: Oyuncunun yakalandığını terminalde görelim
                            println!("[ESP] Oyuncu Yakalandı -> Sağlık: {}, Takım: {}", player_health, player_team);

                            let scene_node = mem.read_pointer(player_pawn + offset::M_P_GAME_SCENE_NODE);
                            if scene_node == 0 { continue; }

                            let pos: [f32; 3] = mem.read(scene_node + offset::M_V_ABS_ORIGIN);
                            let mut head_pos = pos;
                            head_pos[2] += 75.0;

                            if let (Some(feet), Some(head)) = (
                                MemoryManager::world_to_screen(&view_matrix, &pos, screen_size.width(), screen_size.height()),
                                MemoryManager::world_to_screen(&view_matrix, &head_pos, screen_size.width(), screen_size.height()),
                            ) {
                                let height = (feet[1] - head[1]).abs();
                                let width = height / 2.0;
                                let rect = egui::Rect::from_min_max(
                                    egui::pos2(head[0] - width / 2.0, head[1]),
                                    egui::pos2(feet[0] + width / 2.0, feet[1]),
                                );

                                if self.box_esp {
                                    painter.rect_stroke(
                                        rect,
                                        0.0,
                                        egui::Stroke::new(1.5_f32, egui::Color32::from_rgb(255, 0, 0)),
                                    );
                                }

                                if self.health_bar {
                                    let bar_height = height * (player_health as f32 / 100.0);
                                    let bar_rect = egui::Rect::from_min_max(
                                        egui::pos2(rect.min.x - 6.0, rect.max.y - bar_height),
                                        egui::pos2(rect.min.x - 3.0, rect.max.y),
                                    );
                                    painter.rect_filled(bar_rect, 0.0, egui::Color32::GREEN);
                                }
                            }
                        }
                    }
                }
            }
        }

        // --- MENÜ ARAYÜZÜ ---
        if is_open {
            egui::Window::new("MesneviOverlay")
                .title_bar(false)
                .default_pos([100.0, 100.0])
                .resizable(false)
                .show(ctx, |ui| {
                    ui.vertical_centered(|ui| {
                        ui.heading("MESNEVI EXTERNAL");
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
