use windows::Win32::Foundation::HANDLE;
use windows::Win32::System::Diagnostics::ToolHelp::{
    CreateToolhelp32Snapshot, Module32FirstW, Module32NextW, Process32FirstW, Process32NextW,
    MODULEENTRY32W, PROCESSENTRY32W, TH32CS_SNAPMODULE, TH32CS_SNAPMODULE32, TH32CS_SNAPPROCESS,
};
use windows::Win32::System::Memory::ReadProcessMemory;
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

    pub fn get_local_player(&self) -> usize {
        let ptr = self.client_base + crate::offsets::DW_LOCAL_PLAYER_CONTROLLER;
        self.read_pointer(ptr)
    }
}
