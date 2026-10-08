using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Win32.SafeHandles;

// Windows-only long source-check owner. Native gates keep their separate 60/120s
// nativeguard limits. Assignment of this unnamed kill-on-close Job precedes
// ResumeThread; no unrelated process is enumerated or terminated.
public static class GocodeReleaseProcess {
    [StructLayout(LayoutKind.Sequential)] struct SA { public int Length; public IntPtr Descriptor; public int Inherit; }
    [StructLayout(LayoutKind.Sequential)] struct Limits { public long User, JobUser; public uint Flags; public UIntPtr Min, Max; public uint Count; public UIntPtr Affinity; public uint Priority, Schedule; }
    [StructLayout(LayoutKind.Sequential)] struct IO { public ulong RO, WO, OO, RB, WB, OB; }
    [StructLayout(LayoutKind.Sequential)] struct Extended { public Limits Basic; public IO IO; public UIntPtr ProcessMemory, JobMemory, PeakProcess, PeakJob; }
    [StructLayout(LayoutKind.Sequential)] struct Accounting { public long User, Kernel, PeriodUser, PeriodKernel; public uint Faults, Total, Active, Terminated; }
    [StructLayout(LayoutKind.Sequential)] struct SI { public uint Size; public IntPtr Reserved, Desktop, Title; public uint X, Y, Width, Height, XChars, YChars, Fill, Flags; public ushort Show, Extra; public IntPtr ExtraPtr, Input, Output, Error; }
    [StructLayout(LayoutKind.Sequential)] struct SIEX { public SI Startup; public IntPtr Attributes; }
    [StructLayout(LayoutKind.Sequential)] struct PI { public IntPtr Process, Thread; public uint PID, TID; }
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern IntPtr CreateJobObjectW(IntPtr a, string name);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool SetInformationJobObject(IntPtr job, int kind, ref Extended value, uint size);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool QueryInformationJobObject(IntPtr job, int kind, out Accounting value, uint size, IntPtr length);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool TerminateJobObject(IntPtr job, uint code);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool TerminateProcess(IntPtr process, uint code);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool CloseHandle(IntPtr handle);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool CreatePipe(out IntPtr read, out IntPtr write, ref SA attributes, uint size);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool SetHandleInformation(IntPtr handle, uint mask, uint value);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool InitializeProcThreadAttributeList(IntPtr attributes, int count, uint flags, ref UIntPtr bytes);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool UpdateProcThreadAttribute(IntPtr attributes, uint flags, IntPtr key, IntPtr value, UIntPtr bytes, IntPtr old, IntPtr required);
    [DllImport("kernel32.dll")] static extern void DeleteProcThreadAttributeList(IntPtr attributes);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern bool CreateProcessW(string image, StringBuilder command, IntPtr pa, IntPtr ta, bool inherit, uint flags, IntPtr environment, string directory, ref SIEX startup, out PI info);
    [DllImport("kernel32.dll", SetLastError=true)] static extern uint ResumeThread(IntPtr thread);
    [DllImport("kernel32.dll", SetLastError=true)] static extern uint WaitForSingleObject(IntPtr handle, uint milliseconds);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool GetExitCodeProcess(IntPtr process, out uint code);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern int CompareStringOrdinal(string first, int firstLength, string second, int secondLength, bool ignoreCase);

    public sealed class Result {
        public int PID, ExitCode;
        public long ElapsedMS;
        public bool RootReaped, TreeClosed, JobAssignedBeforeResume, TimedOut, OutputLimit;
        public byte[] Stdout = new byte[0], Stderr = new byte[0];
        public string Error = "";
    }
    sealed class Capture {
        public readonly MemoryStream Bytes = new MemoryStream();
        public int Overflow;
        public Task Reader;
    }
    static void Check(bool ok, string action) { if (!ok) throw new Win32Exception(Marshal.GetLastWin32Error(), action); }
    static void Close(ref IntPtr handle) { if (handle != IntPtr.Zero) { Check(CloseHandle(handle), "close owned handle"); handle = IntPtr.Zero; } }
    public static string Quote(string value) {
        if (value == null || value.IndexOf('\0') >= 0) throw new ArgumentException("invalid argument");
        var result = new StringBuilder("\""); int slashes = 0;
        foreach (char ch in value) {
            if (ch == '\\') { slashes++; continue; }
            result.Append('\\', ch == '"' ? 2 * slashes + 1 : slashes); result.Append(ch); slashes = 0;
        }
        return result.Append('\\', 2 * slashes).Append('"').ToString();
    }
    static int CompareNames(string first, string second) {
        int compared = CompareStringOrdinal(first, first.Length, second, second.Length, true);
        if (compared == 0) throw new Win32Exception(Marshal.GetLastWin32Error(), "compare environment names");
        return compared - 2;
    }
    public static string EnvironmentBlock(IDictionary<string,string> environment) {
        if (environment == null || environment.Count > 4096) throw new ArgumentException("bounded environment required");
        var entries = environment.ToList();
        foreach (var entry in entries) {
            string name = entry.Key;
            bool pseudo = name.Length == 3 && name[0] == '=' && char.IsLetter(name[1]) && name[2] == ':';
            if (name.Length == 0 || name.IndexOf('\0') >= 0 || (!pseudo && name.IndexOf('=') >= 0) || entry.Value == null || entry.Value.IndexOf('\0') >= 0)
                throw new ArgumentException("invalid environment name/value");
        }
        // Stable last-value deduplication matches exec.Cmd override semantics.
        var unique = new List<KeyValuePair<string,string>>();
        foreach (var entry in entries) {
            int previous = unique.FindIndex(item => CompareNames(item.Key, entry.Key) == 0);
            if (previous >= 0) unique[previous] = entry; else unique.Add(entry);
        }
        unique.Sort((a,b) => CompareNames(a.Key,b.Key));
        string block = string.Join("\0", unique.Select(item => item.Key + "=" + item.Value)) + "\0\0";
        if (block.Length > 524288) throw new ArgumentException("environment exceeds one MiB");
        return block;
    }
    static Capture Drain(IntPtr handle) {
        var capture = new Capture();
        capture.Reader = Task.Run(() => {
            using (var stream = new FileStream(new SafeFileHandle(handle, true), FileAccess.Read, 8192, false)) {
                byte[] block = new byte[8192]; int count;
                while ((count = stream.Read(block, 0, block.Length)) > 0) {
                    int accepted = Math.Min(count, 4194304 - (int)capture.Bytes.Length);
                    capture.Bytes.Write(block, 0, accepted);
                    if (accepted < count) Interlocked.Exchange(ref capture.Overflow, 1);
                }
            }
        });
        return capture;
    }
    public static Result Run(string image, string[] arguments, string directory, IDictionary<string,string> environment, int timeoutMS) {
        if (!Path.IsPathRooted(image) || !File.Exists(image) || !Directory.Exists(directory) || arguments == null || arguments.Length > 64 || timeoutMS < 1 || timeoutMS > 1200000)
            throw new ArgumentException("invalid bounded owned process request");
        var result = new Result(); var elapsed = Stopwatch.StartNew();
        IntPtr job=IntPtr.Zero, process=IntPtr.Zero, thread=IntPtr.Zero, inputRead=IntPtr.Zero, inputWrite=IntPtr.Zero, outRead=IntPtr.Zero, outWrite=IntPtr.Zero, errRead=IntPtr.Zero, errWrite=IntPtr.Zero, attrs=IntPtr.Zero, handles=IntPtr.Zero, block=IntPtr.Zero;
        Capture stdout=null, stderr=null; bool attrsReady=false;
        try {
            job=CreateJobObjectW(IntPtr.Zero,null); Check(job!=IntPtr.Zero,"create owned Job");
            var limits=new Extended(); limits.Basic.Flags=0x2000;
            Check(SetInformationJobObject(job,9,ref limits,(uint)Marshal.SizeOf(typeof(Extended))),"set kill-on-close Job");
            var sa=new SA{Length=Marshal.SizeOf(typeof(SA)),Inherit=1};
            Check(CreatePipe(out inputRead,out inputWrite,ref sa,0),"create stdin");
            Check(CreatePipe(out outRead,out outWrite,ref sa,0),"create stdout");
            Check(CreatePipe(out errRead,out errWrite,ref sa,0),"create stderr");
            foreach(IntPtr handle in new[]{inputWrite,outRead,errRead}) Check(SetHandleInformation(handle,1,0),"exclude parent pipe handles");
            UIntPtr size=UIntPtr.Zero; InitializeProcThreadAttributeList(IntPtr.Zero,1,0,ref size);
            attrs=Marshal.AllocHGlobal(checked((int)size.ToUInt64())); Check(InitializeProcThreadAttributeList(attrs,1,0,ref size),"initialize handle list"); attrsReady=true;
            handles=Marshal.AllocHGlobal(3*IntPtr.Size);
            Marshal.WriteIntPtr(handles,0,inputRead); Marshal.WriteIntPtr(handles,IntPtr.Size,outWrite); Marshal.WriteIntPtr(handles,2*IntPtr.Size,errWrite);
            Check(UpdateProcThreadAttribute(attrs,0,new IntPtr(0x20002),handles,new UIntPtr((uint)(3*IntPtr.Size)),IntPtr.Zero,IntPtr.Zero),"restrict inherited handles");
            block=Marshal.StringToHGlobalUni(EnvironmentBlock(environment));
            var startup=new SIEX(); startup.Startup.Size=(uint)Marshal.SizeOf(typeof(SIEX)); startup.Startup.Flags=0x100; startup.Startup.Input=inputRead;startup.Startup.Output=outWrite;startup.Startup.Error=errWrite;startup.Attributes=attrs;
            string command=Quote(image)+" "+string.Join(" ",arguments.Select(Quote));
            if(command.Length>32766) throw new ArgumentException("command exceeds Windows bound");
            PI info; Check(CreateProcessW(image,new StringBuilder(command),IntPtr.Zero,IntPtr.Zero,true,0x08000000|0x80000|0x400|4,block,directory,ref startup,out info),"create suspended owned process");
            process=info.Process;thread=info.Thread;result.PID=checked((int)info.PID);
            if(!AssignProcessToJobObject(job,process)) {
                int error=Marshal.GetLastWin32Error(); Check(TerminateProcess(process,1),"cancel unassigned suspended process");
                if(WaitForSingleObject(process,3000)!=0) throw new TimeoutException("unassigned suspended process did not reap");
                result.RootReaped=true; throw new Win32Exception(error,"assign Job before Resume");
            }
            result.JobAssignedBeforeResume=true;
            stdout=Drain(outRead);outRead=IntPtr.Zero;stderr=Drain(errRead);errRead=IntPtr.Zero;
            Check(ResumeThread(thread)!=uint.MaxValue,"resume owned process");
            Close(ref inputRead);Close(ref inputWrite);Close(ref outWrite);Close(ref errWrite);Close(ref thread);
            while(true) {
                uint wait=WaitForSingleObject(process,25);
                if(wait==0){result.RootReaped=true;break;}
                if(wait!=258) throw new Win32Exception(Marshal.GetLastWin32Error(),"wait owned process");
                if(Volatile.Read(ref stdout.Overflow)!=0||Volatile.Read(ref stderr.Overflow)!=0){result.OutputLimit=true;break;}
                if(elapsed.ElapsedMilliseconds>=timeoutMS){result.TimedOut=true;break;}
            }
        } catch(Exception error) { result.Error=error.Message; }
        finally {
            try {
                Close(ref inputRead);Close(ref inputWrite);Close(ref outWrite);Close(ref errWrite);Close(ref thread);
                if(job!=IntPtr.Zero) {
                    Check(TerminateJobObject(job,2),"terminate only owned Job");
                    var retirement=Stopwatch.StartNew(); Accounting accounting;
                    do {
                        Check(QueryInformationJobObject(job,1,out accounting,(uint)Marshal.SizeOf(typeof(Accounting)),IntPtr.Zero),"query owned descendants");
                        if(accounting.Active==0)break;
                        Thread.Sleep(10);
                    } while(retirement.ElapsedMilliseconds<3000);
                    if(accounting.Active!=0)throw new TimeoutException("owned descendants did not retire within three seconds");
                    if(process!=IntPtr.Zero) {
                        if(WaitForSingleObject(process,(uint)Math.Max(0,3000-retirement.ElapsedMilliseconds))!=0)throw new TimeoutException("owned root did not reap");
                        result.RootReaped=true;uint code;Check(GetExitCodeProcess(process,out code),"read actual owned exit");result.ExitCode=unchecked((int)code);
                    }
                    Close(ref job);result.TreeClosed=true;
                }
            } catch(Exception error) { result.Error=Join(result.Error,error.Message); }
            try {
                if(stdout!=null&&stderr!=null) {
                    if(!Task.WaitAll(new[]{stdout.Reader,stderr.Reader},3000))throw new TimeoutException("owned pipe readers did not join within three seconds");
                    result.Stdout=stdout.Bytes.ToArray();result.Stderr=stderr.Bytes.ToArray();
                    result.OutputLimit|=stdout.Overflow!=0||stderr.Overflow!=0;
                }
            } catch(Exception error) { result.Error=Join(result.Error,error.Message); }
            // Close the kill-on-close Job even if a retirement proof failed.
            Close(ref job);Close(ref process);Close(ref outRead);Close(ref errRead);
            if(attrs!=IntPtr.Zero){if(attrsReady)DeleteProcThreadAttributeList(attrs);Marshal.FreeHGlobal(attrs);}
            if(handles!=IntPtr.Zero)Marshal.FreeHGlobal(handles);if(block!=IntPtr.Zero)Marshal.FreeHGlobal(block);
            result.ElapsedMS=elapsed.ElapsedMilliseconds;
            if(result.TimedOut)result.Error=Join(result.Error,"owned process deadline expired");
            if(result.OutputLimit)result.Error=Join(result.Error,"owned process output exceeded four MiB per stream");
        }
        return result;
    }
    static string Join(string before,string next){return before.Length==0?next:before+"; "+next;}
}

// Validate structure before PowerShell conversion, whose duplicate-property
// handling varies by PowerShell version. This never evaluates JSON as code.
public static class GocodeReleaseJson {
    public static void Validate(string text) { new Parser(text).Validate(); }
    sealed class Parser {
        readonly string text; int at;
        public Parser(string value) { if(value==null||Encoding.UTF8.GetByteCount(value)>524288)throw new InvalidDataException("JSON exceeds512KiB");text=value; }
        public void Validate(){Value(0);White();if(at!=text.Length)Fail();}
        void White(){while(at<text.Length&&(text[at]==' '||text[at]=='\t'||text[at]=='\n'||text[at]=='\r'))at++;}
        void Fail(){throw new InvalidDataException("invalid/duplicate bounded JSON at"+at);}
        bool Take(char ch){White();if(at<text.Length&&text[at]==ch){at++;return true;}return false;}
        void Need(char ch){if(!Take(ch))Fail();}
        void Value(int depth){
            if(depth>64)Fail();White();if(at>=text.Length)Fail();char ch=text[at];
            if(ch=='{'){
                at++;var keys=new HashSet<string>(StringComparer.Ordinal);if(Take('}'))return;
                do{White();string key=String();if(!keys.Add(key))Fail();Need(':');Value(depth+1);if(Take('}'))return;Need(',');}while(true);
            }
            if(ch=='['){at++;if(Take(']'))return;do{Value(depth+1);if(Take(']'))return;Need(',');}while(true);}
            if(ch=='"'){String();return;}
            foreach(string token in new[]{"true","false","null"}){if(at+token.Length<=text.Length&&text.Substring(at,token.Length)==token){at+=token.Length;return;}}
            Number();
        }
        string String(){
            if(at>=text.Length||text[at++]!='"'){Fail();}var result=new StringBuilder();
            while(at<text.Length){char ch=text[at++];if(ch=='"')return result.ToString();if(ch<32)Fail();
                if(ch!='\\'){result.Append(ch);continue;}if(at>=text.Length)Fail();ch=text[at++];
                switch(ch){case '"':case '\\':case '/':result.Append(ch);break;case 'b':result.Append('\b');break;case 'f':result.Append('\f');break;case 'n':result.Append('\n');break;case 'r':result.Append('\r');break;case 't':result.Append('\t');break;case 'u':
                    int value=0;for(int i=0;i<4;i++){if(at>=text.Length)Fail();char digit=text[at++];int v=digit>='0'&&digit<='9'?digit-'0':digit>='a'&&digit<='f'?digit-'a'+10:digit>='A'&&digit<='F'?digit-'A'+10:-1;if(v<0)Fail();value=value*16+v;}result.Append((char)value);break;
                    default:Fail();break;}
            }Fail();return null;
        }
        void Number(){
            int start=at;if(at<text.Length&&text[at]=='-')at++;
            if(at<text.Length&&text[at]=='0')at++;else{if(at>=text.Length||text[at]<'1'||text[at]>'9')Fail();while(at<text.Length&&text[at]>='0'&&text[at]<='9')at++;}
            if(at<text.Length&&text[at]=='.'){at++;int digits=at;while(at<text.Length&&text[at]>='0'&&text[at]<='9')at++;if(at==digits)Fail();}
            if(at<text.Length&&(text[at]=='e'||text[at]=='E')){at++;if(at<text.Length&&(text[at]=='+'||text[at]=='-'))at++;int digits=at;while(at<text.Length&&text[at]>='0'&&text[at]<='9')at++;if(at==digits)Fail();}
            if(start==at)Fail();
        }
    }
}
