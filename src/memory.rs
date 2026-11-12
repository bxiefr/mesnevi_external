use windows::Win32::Foundation::HANDLE;
use windows::Win32::System::Diagnostics::Debug::ReadProcessMemory;
use windows::Win32::System::Diagnostics::ToolHelp::{
    CreateToolhelp32Snapshot, Module32FirstW, Module32NextW, Process32FirstW, Process32NextW,
    MODULEENTRY32W, PROCESSENTRY32W, TH32CS_SNAPMODULE, TH32CS_SNAPMODULE32, TH32CS_SNAPPROCESS,
};
use windows::Win32::System::Threading::{OpenProcess, PROCESS_VM_READ};

pub struct MemoryManager {
    pub pid: u32,
    pub handle: HANDLE,
    pub client_base: usize,
}

impl MemoryManager {
    pub fn new(process_name: &str) -> Option<Self> {
        unsafe {
            let pid = Self::get_process_id(process_name)?;
            let handle = OpenProcess(PROCESS_VM_READ, false, pid).ok()?;
            let client_base = Self::get_module_base(pid, "client.dll")?;

            Some(Self {
                pid,
                handle,
                client_base,
            })
        }
    }

    unsafe fn get_process_id(process_name: &str) -> Option<u32> {
        let snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0).ok()?;
        let mut entry = PROCESSENTRY32W::default();
        entry.dwSize = std::mem::size_of::<PROCESSENTRY32W>() as u32;

        if Process32FirstW(snapshot, &mut entry).is_ok() {
            loop {
                let name = String::from_utf16_lossy(
                    &entry.szExeFile[..entry.szExeFile.iter().position(|&c| c == 0).unwrap_or(entry.szExeFile.len())],
                );
                if name.eq_ignore_ascii_case(process_name) {
                    return Some(entry.th32ProcessID);
                }
                if !Process32NextW(snapshot, &mut entry).is_ok() {
                    break;
                }
            }
        }
        None
    }

    unsafe fn get_module_base(pid: u32, module_name: &str) -> Option<usize> {
        let snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPMODULE | TH32CS_SNAPMODULE32, pid).ok()?;
        let mut entry = MODULEENTRY32W::default();
        entry.dwSize = std::mem::size_of::<MODULEENTRY32W>() as u32;

        if Module32FirstW(snapshot, &mut entry).is_ok() {
            loop {
                let name = String::from_utf16_lossy(
                    &entry.szModule[..entry.szModule.iter().position(|&c| c == 0).unwrap_or(entry.szModule.len())],
                );
                if name.eq_ignore_ascii_case(module_name) {
                    return Some(entry.modBaseAddr as usize);
                }
                if Module32NextW(snapshot, &mut entry).is_err() {
                    break;
                }
            }
        }
        None
    }

    pub fn read<T: Copy>(&self, address: usize) -> T {
        let mut buffer: T = unsafe { std::mem::zeroed() };
        unsafe {
            let _ = ReadProcessMemory(
                self.handle,
                address as *const _,
                &mut buffer as *mut T as *mut _,
                std::mem::size_of::<T>(),
                None,
            );
        }
        buffer
    }

    pub fn read_pointer(&self, address: usize) -> usize {
        self.read::<usize>(address)
    }

    pub fn get_view_matrix(&self) -> [f32; 16] {
        self.read::<[f32; 16]>(self.client_base + crate::offset::DW_VIEW_MATRIX)
    }

    pub fn get_local_player_controller(&self) -> usize {
        self.read_pointer(self.client_base + crate::offset::DW_LOCAL_PLAYER_CONTROLLER)
    }

    pub fn world_to_screen(matrix: &[f32; 16], pos: &[f32; 3], width: f32, height: f32) -> Option<[f32; 2]> {
        let w = matrix[12] * pos[0] + matrix[13] * pos[1] + matrix[14] * pos[2] + matrix[15];
        if w < 0.01 {
            return None;
        }

        let inv_w = 1.0 / w;
        let x = matrix[0] * pos[0] + matrix[1] * pos[1] + matrix[2] * pos[2] + matrix[3];
        let y = matrix[4] * pos[0] + matrix[5] * pos[1] + matrix[6] * pos[2] + matrix[7];

        let screen_x = (width / 2.0) + (0.5 * x * inv_w * width + 0.5);
        let screen_y = (height / 2.0) - (0.5 * y * inv_w * height + 0.5);

        Some([screen_x, screen_y])
    }
}
